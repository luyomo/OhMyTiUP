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
	// "errors"
	"fmt"

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	// "github.com/aws/aws-sdk-go-v2/service/iam/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateTemplate(pexecutor *ctxt.Executor, subClusterType string, network NetworkType) *Builder {
	b.tasks = append(b.tasks, &CreateTemplate{BaseTemplate: BaseTemplate{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}}})
	return b
}

func (b *Builder) ListTemplate(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListTemplate{
		BaseTemplate: BaseTemplate{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

func (b *Builder) DestroyTemplate(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyTemplate{
		BaseTemplate: BaseTemplate{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

/******************************************************************************/

type Templates struct {
	BaseResourceInfo
}

func (d *Templates) ToPrintTable() *[][]string {
	tableTemplate := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(Template)
		// tableTemplate = append(tableTemplate, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableTemplate
}

func (d *Templates) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		// return _data.(types.ConnectorSummary).ConnectorArn, nil
		return nil, nil
	})
}

/******************************************************************************/
type BaseTemplate struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *iam.Client // Replace the example to specific service
}

func (b *BaseTemplate) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = iam.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &Templates{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseTemplate) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	// Pattern01: List

	// TODO: Replace if necessary
	// filters := b.MakeEC2Filters()

	resp, err := b.client.ListPolicies(context.TODO(), &iam.ListPoliciesInput{})
	if err != nil {
		return err
	}

	for _, policy := range resp.Policies {
		b.ResourceData.Append(policy)
	}

	// Pattern02: Descibe using filters
	// filters := b.MakeEC2Filters()

	// for _, label := range b.awsTopoConfigs.Labels {
	// 	*filters = append(*filters, types.Filter{Name: aws.String("tag:label:" + label.Name), Values: []string{label.Value}})
	// }

	// resp, err := b.client.DescribeLaunchTemplates(context.TODO(), &ec2.DescribeLaunchTemplatesInput{Filters: *filters})
	// if err != nil {
	// 	return err
	// }

	// for _, launchTemplate := range resp.LaunchTemplates {
	// 	b.ResourceData.Append(launchTemplate)
	// }

	// Pattern03: Describe
	// resp, err := b.client.DescribeLoadBalancers(context.TODO(), &nlb.DescribeLoadBalancersInput{Names: []string{c.clusterName}})
	// if err != nil {
	// 	var ae smithy.APIError
	// 	if errors.As(err, &ae) {
	// 		fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
	// 		if ae.ErrorCode() == "LoadBalancerNotFound" {
	// 			return nil, nil
	// 		}
	// 	}

	// 	return nil, err
	// }

	// for _, loadBalancer := range resp.LoadBalancers {
	// 	b.ResourceData.Append(loadBalancer)
	// }
	return nil
}

/******************************************************************************/
type CreateTemplate struct {
	BaseTemplate

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTemplate) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// TODO: Add resource preparation
		// Pattern01:
		// tags := []types.Tag{
		// 	{Key: aws.String("Name"), Value: aws.String(c.clusterName)},
		// 	{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)},
		// 	{Key: aws.String("Type"), Value: aws.String("glue")},
		// 	{Key: aws.String("Component"), Value: aws.String("kafkaconnect")},
		// }

		// if _, err = c.client.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{}); err != nil {
		// 	return err
		// }

		// *************************************************************
		// Pattern02:
		// tags := c.MakeEC2Tags()

		// if _, err = c.client.CreateRouteTable(context.TODO(), &ec2.CreateRouteTableInput{
		// 	VpcId: vpcId,
		// 	TagSpecifications: []types.TagSpecification{
		// 		types.TagSpecification{
		// 			ResourceType: types.ResourceTypeSecurityGroup,
		// 			Tags:         *tags,
		// 		},
		// 	},
		// }); err != nil {
		// 	return err
		// }

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTemplate) String() string {
	return fmt.Sprintf("Echo: Create Template ... ...  ")
}

type DestroyTemplate struct {
	BaseTemplate
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyTemplate) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyTemplate ****** \n\n\n")

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		// TODO: Destroy the cluster
		// _id, err := c.ResourceData.GetResourceArn()
		// if err != nil {
		// 	return err
		// }
		// if _, err = c.client.CreateRouteTable(context.TODO(), &ec2.CreateRouteTableInput{
		// 	RouteTableId: _id,
		// }); err != nil {
		// 	return err
		// }

	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTemplate) String() string {
	return fmt.Sprintf("Echo: Destroying Template")
}

type ListTemplate struct {
	BaseTemplate
}

// Execute implements the Task interface
func (c *ListTemplate) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListTemplate ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTemplate) String() string {
	return fmt.Sprintf("Echo: List  ")
}
