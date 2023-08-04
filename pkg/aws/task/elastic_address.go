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

	// "github.com/aws/aws-sdk-go-v2/aws"
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
func (b *Builder) CreateElasticAddress(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateElasticAddress{BaseElasticAddress: BaseElasticAddress{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypeNAT}}})
	return b
}

func (b *Builder) ListElasticAddress(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListElasticAddress{BaseElasticAddress: BaseElasticAddress{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

func (b *Builder) DestroyElasticAddress(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyElasticAddress{BaseElasticAddress: BaseElasticAddress{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}}})
	return b
}

/******************************************************************************/

type ElasticAddresss struct {
	BaseResourceInfo
}

func (d *ElasticAddresss) ToPrintTable() *[][]string {
	tableElasticAddress := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(ElasticAddress)
		// tableElasticAddress = append(tableElasticAddress, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableElasticAddress
}

func (d *ElasticAddresss) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		if throwErr == ThrowErrorIfNotExists {
			return nil, errors.New("No resource(elastic address) found")
		} else {
			return nil, nil
		}
	}

	return (d.Data[0]).(types.Address).AllocationId, nil
}

/******************************************************************************/
type BaseElasticAddress struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
	region *string
}

func (b *BaseElasticAddress) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
	b.region = &cfg.Region

	b.client = ec2.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &ElasticAddresss{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseElasticAddress) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeAddresses(context.TODO(), &ec2.DescribeAddressesInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, address := range resp.Addresses {
		b.ResourceData.Append(address)
	}
	return nil
}

/******************************************************************************/
type CreateElasticAddress struct {
	BaseElasticAddress
}

// Execute implements the Task interface
func (c *CreateElasticAddress) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		tags := c.MakeEC2Tags()

		if _, err = c.client.AllocateAddress(context.TODO(), &ec2.AllocateAddressInput{
			NetworkBorderGroup: c.region,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeElasticIp,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateElasticAddress) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateElasticAddress) String() string {
	return fmt.Sprintf("Echo: Create ElasticAddress ... ...  ")
}

type DestroyElasticAddress struct {
	BaseElasticAddress
}

// Execute implements the Task interface
func (c *DestroyElasticAddress) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	_id, err := c.ResourceData.GetResourceArn(ContinueIfNotExists)
	if err != nil {
		return err
	}

	if _id != nil {
		if _, err = c.client.ReleaseAddress(context.TODO(), &ec2.ReleaseAddressInput{
			AllocationId: _id,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyElasticAddress) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyElasticAddress) String() string {
	return fmt.Sprintf("Echo: Destroying ElasticAddress")
}

type ListElasticAddress struct {
	BaseElasticAddress
}

// Execute implements the Task interface
func (c *ListElasticAddress) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListElasticAddress ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListElasticAddress) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListElasticAddress) String() string {
	return fmt.Sprintf("Echo: List  ")
}
