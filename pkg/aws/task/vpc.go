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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
// func (b *Builder) CreateVpc(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
// 	b.tasks = append(b.tasks, &CreateVpc{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 		clusterInfo:    clusterInfo,
// 	})
// 	return b
// }

// func (b *Builder) DestroyVpc(pexecutor *ctxt.Executor, subClusterType string) *Builder {
// 	b.tasks = append(b.tasks, &DestroyVpc{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 	})
// 	return b
// }

//	func (b *Builder) ListVpc(pexecutor *ctxt.Executor, tableVPC *[][]string) *Builder {
//		b.tasks = append(b.tasks, &ListVpc{
//			pexecutor: pexecutor,
//			tableVPC:  tableVPC,
//		})
//		return b
//	}
func (b *Builder) CreateVPC(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateVPC{
		BaseVPC:     BaseVPC{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}},
		clusterInfo: clusterInfo,
	})
	return b
}

func (b *Builder) ListVPC(pexecutor *ctxt.Executor, tableVPC *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListVPC{
		BaseVPC:  BaseVPC{BaseTask: BaseTask{pexecutor: pexecutor}},
		tableVPC: tableVPC,
	})
	return b
}

func (b *Builder) DestroyVPC(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyVPC{
		BaseVPC: BaseVPC{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

/******************************************************************************/

type VPCs struct {
	BaseResourceInfo
}

func (d *VPCs) ToPrintTable() *[][]string {
	tableVPC := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(VPC)
		// tableVPC = append(tableVPC, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableVPC
}

func (d *VPCs) GetResourceArn() (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource found - TODO: replace name")
	}

	// return (d.Data[0]).(*types.Role).Arn, nil
	return nil, nil
}

/******************************************************************************/
type BaseVPC struct {
	BaseTask

	ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
	// subClusterType string
}

func (b *BaseVPC) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = ec2.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &VPCs{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseVPC) readResources() error {

	var filters []types.Filter
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{b.clusterName}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{b.clusterType}})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{b.subClusterType}})
	}

	resp, err := b.client.DescribeVpcs(context.TODO(), &ec2.DescribeVpcsInput{Filters: filters})
	if err != nil {
		return err
	}

	for _, vpc := range resp.Vpcs {
		b.ResourceData.Append(vpc)
	}
	return nil
}

func (b *BaseVPC) GetVPCItem(itemType string) (*string, error) {
	resourceExistFlag, err := b.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}

	if resourceExistFlag == false {
		return nil, errors.New("No VPC found")
	}

	_data := b.ResourceData.GetData()

	if itemType == "VpcId" {
		return _data[0].(types.Vpc).VpcId, nil
	} else if itemType == "CidrBlock" {
		return _data[0].(types.Vpc).CidrBlock, nil
	} else if itemType == "State" {
		state := string(_data[0].(types.Vpc).State)
		return &state, nil
	}

	return nil, errors.New(fmt.Sprintf("not support item from vpc", itemType))

}

func (b *BaseVPC) GetVpcID() (*string, error) {
	resourceExistFlag, err := b.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}

	if resourceExistFlag == false {
		return nil, errors.New("No VPC found")
	}

	_data := b.ResourceData.GetData()
	return _data[0].(types.Vpc).VpcId, nil
}

/******************************************************************************/
type CreateVPC struct {
	BaseVPC

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateVPC) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// TODO: Add resource preparation

		// tags := []types.Tag{
		// 	{Key: aws.String("Name"), Value: aws.String(c.clusterName)},
		// 	{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)},
		// 	{Key: aws.String("Type"), Value: aws.String("glue")},
		// 	{Key: aws.String("Component"), Value: aws.String("kafkaconnect")},
		// }

		// if _, err = c.client.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{}); err != nil {
		// 	return err
		// }

		// TODO: Check cluster status until expected status
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateVPC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateVPC) String() string {
	return fmt.Sprintf("Echo: Create VPC ... ...  ")
}

type DestroyVPC struct {
	BaseVPC
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyVPC) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyVPC ****** \n\n\n")

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
func (c *DestroyVPC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVPC) String() string {
	return fmt.Sprintf("Echo: Destroying VPC")
}

type ListVPC struct {
	BaseVPC

	tableVPC *[][]string
}

// Execute implements the Task interface
func (c *ListVPC) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListVPC ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListVPC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListVPC) String() string {
	return fmt.Sprintf("Echo: List  ")
}
