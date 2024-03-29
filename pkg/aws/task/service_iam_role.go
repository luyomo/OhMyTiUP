// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"

	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateServiceIamRole(subClusterType, policyDocument string) *Builder {
	b.tasks = append(b.tasks, &CreateServiceIamRole{policyDocument: policyDocument, BaseServiceIamRole: BaseServiceIamRole{BaseTask: BaseTask{subClusterType: subClusterType}}})
	return b
}

func (b *Builder) ListServiceIamRole() *Builder {
	b.tasks = append(b.tasks, &ListServiceIamRole{})
	return b
}

func (b *Builder) DestroyServiceIamRole() *Builder {
	b.tasks = append(b.tasks, &DestroyServiceIamRole{})
	return b
}

/******************************************************************************/

type ServiceIamRoles struct {
	BaseResourceInfo
}

func (d *ServiceIamRoles) ToPrintTable() *[][]string {
	tableServiceIamRole := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(ServiceIamRole)
		// tableServiceIamRole = append(tableServiceIamRole, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableServiceIamRole
}

func (d *ServiceIamRoles) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.Role).Arn, nil
	})
}

/******************************************************************************/
type BaseServiceIamRole struct {
	BaseTask

	ResourceData ResourceData
	roleName     string
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *iam.Client // Replace the example to specific service
}

func (b *BaseServiceIamRole) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	fmt.Printf("Searching role name: %s.%s \n\n\n", b.clusterName, b.subClusterType)
	b.roleName = fmt.Sprintf("%s.%s", b.clusterName, b.subClusterType)

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = iam.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &ServiceIamRoles{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseServiceIamRole) readResources() error {
	resp, err := b.client.ListRoles(context.TODO(), &iam.ListRolesInput{
		PathPrefix: aws.String(fmt.Sprintf("/%s/", b.subClusterType)),
	})
	if err != nil {
		return err
	}

	for _, role := range resp.Roles {
		if *role.RoleName == b.roleName {
			b.ResourceData.Append(role)
		}
	}
	return nil
}

func (b *BaseServiceIamRole) GetKeyId() (*string, error) {
	_data := b.ResourceData.GetData()
	if len(_data) == 0 {
		return nil, errors.New("No role found")
	}
	return _data[0].(types.Role).Arn, nil

}

/******************************************************************************/
type CreateServiceIamRole struct {
	BaseServiceIamRole

	policyDocument string
}

// Execute implements the Task interface
func (c *CreateServiceIamRole) Execute(ctx context.Context) error {

	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// TODO: Add resource preparation

		tags := []types.Tag{
			{Key: aws.String("Name"), Value: aws.String(c.clusterName)},
			{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)},
			{Key: aws.String("Type"), Value: aws.String(c.subClusterType)},
		}

		if _, err = c.client.CreateRole(context.TODO(), &iam.CreateRoleInput{
			RoleName:                 aws.String(c.roleName),
			Path:                     aws.String(fmt.Sprintf("/%s/", c.subClusterType)),
			Tags:                     tags,
			AssumeRolePolicyDocument: aws.String(c.policyDocument),
			// 				`{
			//     "Version": "2012-10-17",
			//     "Statement": [
			//         {
			//             "Sid": "Statement1",
			//             "Effect": "Allow",
			//             "Principal": {
			//                 "Service": "kafkaconnect.amazonaws.com"
			//             },
			//             "Action": "sts:AssumeRole"
			//         }
			//     ]
			// }`

		}); err != nil {
			return err
		}

		// TODO: Check cluster status until expected status
	}

	if err := c.attachPoliciesToRole(ctx); err != nil {
		return err
	}

	return nil
}

func (c *CreateServiceIamRole) attachPoliciesToRole(ctx context.Context) error {

	// Get policies arn from name
	listServiceIamPolicy := &ListServiceIamPolicy{BaseServiceIamPolicy: BaseServiceIamPolicy{BaseTask: BaseTask{pexecutor: c.pexecutor, clusterName: c.clusterName, subClusterType: c.subClusterType}}}
	if err := listServiceIamPolicy.Execute(ctx); err != nil {
		return err
	}
	policyArn, err := listServiceIamPolicy.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	// List policies from role
	listAttachedRolePolicies, err := c.client.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{
		RoleName:   aws.String(c.roleName),
		PathPrefix: aws.String(fmt.Sprintf("/%s/", c.subClusterType)),
	})

	for _, attachedPolicy := range listAttachedRolePolicies.AttachedPolicies {
		// roleName == policyName : clusterName.subClusterType
		if *attachedPolicy.PolicyName == c.roleName {
			return nil
		}
	}

	if _, err := c.client.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: policyArn,
		RoleName:  aws.String(c.roleName),
	}); err != nil {
		return err
	}

	// If it does not exist, add it
	return nil
}

// Rollback implements the Task interface
func (c *CreateServiceIamRole) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateServiceIamRole) String() string {
	return fmt.Sprintf("Echo: Create ServiceIamRole ... ...  ")
}

type DestroyServiceIamRole struct {
	BaseServiceIamRole
}

// Execute implements the Task interface
func (c *DestroyServiceIamRole) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyServiceIamRole ****** \n\n\n")

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		// TODO: Destroy the cluster
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyServiceIamRole) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyServiceIamRole) String() string {
	return fmt.Sprintf("Echo: Destroying ServiceIamRole")
}

type ListServiceIamRole struct {
	BaseServiceIamRole
}

// Execute implements the Task interface
func (c *ListServiceIamRole) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListServiceIamRole ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListServiceIamRole) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListServiceIamRole) String() string {
	return fmt.Sprintf("Echo: List  ")
}
