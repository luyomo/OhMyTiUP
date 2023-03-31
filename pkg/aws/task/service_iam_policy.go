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
	// "github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateServiceIamPolicy() *Builder {
	b.tasks = append(b.tasks, &CreateServiceIamPolicy{})
	return b
}

func (b *Builder) ListServiceIamPolicy() *Builder {
	b.tasks = append(b.tasks, &ListServiceIamPolicy{})
	return b
}

func (b *Builder) DestroyServiceIamPolicy() *Builder {
	b.tasks = append(b.tasks, &DestroyServiceIamPolicy{})
	return b
}

/******************************************************************************/

type ServiceIamPolicys struct {
	BaseResourceInfo
}

func (d *ServiceIamPolicys) ToPrintTable() *[][]string {
	tableExample := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(types.Policy)
		tableExample = append(tableExample, []string{
			*_entry.PolicyName,
		})
	}
	return &tableExample
}

func (d *ServiceIamPolicys) GetResourceArn() (*string, error) {

	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource(service iam policy) found")
	}

	return (d.Data[0]).(*types.Policy).Arn, nil
}

/******************************************************************************/
type BaseServiceIamPolicy struct {
	BaseTask

	ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *iam.Client // Replace the example to specific service
}

func (b *BaseServiceIamPolicy) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = iam.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &ServiceIamPolicys{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseServiceIamPolicy) readResources() error {
	resp, err := b.client.ListPolicies(context.TODO(), &iam.ListPoliciesInput{
		PathPrefix: aws.String("/kafkaconnect/"),
	})
	if err != nil {
		return err
	}

	for _, policy := range resp.Policies {
		fmt.Printf("<%s> vs <%s> \n\n\n\n\n", *policy.PolicyName, b.clusterName)
		if *policy.PolicyName == b.clusterName {
			fmt.Printf("Coming here to set the calue <%s> \n\n\n\n", *policy.PolicyName)
			b.ResourceData.Append(&policy)
		}
	}
	return nil
}

/******************************************************************************/
type CreateServiceIamPolicy struct {
	BaseServiceIamPolicy

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateServiceIamPolicy) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		tags := []types.Tag{
			{Key: aws.String("Name"), Value: aws.String(c.clusterName)},
			{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)},
			{Key: aws.String("Type"), Value: aws.String("glue")},
			{Key: aws.String("Component"), Value: aws.String("kafkaconnect")},
		}

		policy := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "glue:ListSchemaVersions",
                "glue:GetRegistry",
                "glue:QuerySchemaVersionMetadata",
                "glue:GetSchemaVersionsDiff",
                "glue:ListSchemas",
                "glue:UntagResource",
                "glue:GetSchema",
                "glue:TagResource",
                "glue:GetSchemaByDefinition"
            ],
            "Resource": [
                "arn:aws:glue:*:%s:schema/*",
                "arn:aws:glue:*:%s:registry/*"
            ]
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "glue:GetSchemaVersion",
                "glue:ListRegistries"
            ],
            "Resource": "*"
        }
    ]
}`

		_, accountID, err := GetCallerUser(ctx)
		if err != nil {
			return err
		}

		if _, err = c.client.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
			PolicyName:     aws.String(c.clusterName),
			Tags:           tags,
			Path:           aws.String("/kafkaconnect/"),
			PolicyDocument: aws.String(fmt.Sprintf(policy, accountID, accountID)),
		}); err != nil {
			return err
		}

		// TODO: Check cluster status until expected status
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateServiceIamPolicy) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateServiceIamPolicy) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

type DestroyServiceIamPolicy struct {
	BaseServiceIamPolicy
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyServiceIamPolicy) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyServiceIamPolicy ****** \n\n\n")

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		// Destroy the cluster
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyServiceIamPolicy) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyServiceIamPolicy) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListServiceIamPolicy struct {
	BaseServiceIamPolicy
}

// Execute implements the Task interface
func (c *ListServiceIamPolicy) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListServiceIamPolicy ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListServiceIamPolicy) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListServiceIamPolicy) String() string {
	return fmt.Sprintf("Echo: List Example ")
}
