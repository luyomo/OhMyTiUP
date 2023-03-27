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
type TransitGateway struct {
	TransitGatewayId  string `json:"TransitGatewayId"`
	TransitGatewayArn string `json:"TransitGatewayArn`
	State             string `json:"State"`
}

type TransitGateways struct {
	TransitGateways []TransitGateway `json:"TransitGateways"`
}

func (b *Builder) CreateTransitGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &CreateTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

func (b *Builder) ListTransitGateway(pexecutor *ctxt.Executor, transitGateway *TransitGateway) *Builder {
	b.tasks = append(b.tasks, &ListTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{pexecutor: pexecutor}}, transitGateway: transitGateway})
	return b
}

func (b *Builder) DestroyTransitGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

// func (b *Builder) CreateTransitGateway() *Builder {
// 	b.tasks = append(b.tasks, &CreateTransitGateway{})
// 	return b
// }

// func (b *Builder) ListTransitGateway() *Builder {
// 	b.tasks = append(b.tasks, &ListTransitGateway{})
// 	return b
// }

// func (b *Builder) DestroyTransitGateway() *Builder {
// 	b.tasks = append(b.tasks, &DestroyTransitGateway{})
// 	return b
// }

/******************************************************************************/

type TransitGatewaysInfo struct {
	BaseResourceInfo
}

func (d *TransitGatewaysInfo) ToPrintTable() *[][]string {
	tableTransitGateway := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(TransitGateway)
		// tableTransitGateway = append(tableTransitGateway, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableTransitGateway
}

func (d *TransitGatewaysInfo) GetResourceArn() (*string, error) {
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
type BaseTransitGateway struct {
	BaseTask

	ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseTransitGateway) init(ctx context.Context) error {
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
		b.ResourceData = &TransitGatewaysInfo{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseTransitGateway) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	// TODO: Replace if necessary
	filters := b.MakeEC2Filters()
	fmt.Printf("Filters are: <%#v> \n\n\n\n\n", filters)

	resp, err := b.client.DescribeTransitGateways(context.TODO(), &ec2.DescribeTransitGatewaysInput{
		Filters: *filters,
	})
	if err != nil {
		return err
	}

	for _, transitGateway := range resp.TransitGateways {
		b.ResourceData.Append(transitGateway)
	}
	return nil
}

func (b *BaseTransitGateway) GetTransitGatewayID() (*string, error) {
	resourceExistFlag, err := b.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}

	if resourceExistFlag == false {
		return nil, errors.New("No TransitGateway found")
	}

	_data := b.ResourceData.GetData()

	return _data[0].(types.TransitGateway).TransitGatewayId, nil

}

/******************************************************************************/
type CreateTransitGateway struct {
	BaseTransitGateway

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTransitGateway) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// TODO: Add resource preparation
		tags := c.MakeEC2Tags()

		if _, err = c.client.CreateTransitGateway(context.TODO(), &ec2.CreateTransitGatewayInput{
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeTransitGateway,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}

		// TODO: Check cluster status until expected status
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTransitGateway) String() string {
	return fmt.Sprintf("Echo: Create TransitGateway ... ...  ")
}

type DestroyTransitGateway struct {
	BaseTransitGateway
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyTransitGateway) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyTransitGateway ****** \n\n\n")
	_data := c.ResourceData.GetData()
	for _, transitGateway := range _data {
		_entry := transitGateway.(types.TransitGateway)
		if _, err := c.client.DeleteTransitGateway(context.TODO(), &ec2.DeleteTransitGatewayInput{
			TransitGatewayId: _entry.TransitGatewayId,
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTransitGateway) String() string {
	return fmt.Sprintf("Echo: Destroying TransitGateway")
}

type ListTransitGateway struct {
	BaseTransitGateway

	transitGateway *TransitGateway
}

// Execute implements the Task interface
func (c *ListTransitGateway) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListTransitGateway ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTransitGateway) String() string {
	return fmt.Sprintf("Echo: List  ")
}
