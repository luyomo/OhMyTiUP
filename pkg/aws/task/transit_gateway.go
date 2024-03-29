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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

type TransitGatewayState_Process types.TransitGatewayState

func (p TransitGatewayState_Process) isState(mode ReadResourceMode) bool {
	switch mode {
	case ReadResourceModeCommon:
		return p.isOKState()
	case ReadResourceModeBeforeCreate:
		return p.isBeforeCreateState()
	case ReadResourceModeAfterCreate:
		return p.isAfterCreateState()
	case ReadResourceModeBeforeDestroy:
		return p.isBeforeDestroyState()
	case ReadResourceModeAfterDestroy:
		return p.isAfterDestroyState()
	}
	return true
}

func (p TransitGatewayState_Process) isBeforeCreateState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayStatePending),
		string(types.TransitGatewayStateAvailable),
		string(types.TransitGatewayStateModifying),
	}, string(p))

}

func (p TransitGatewayState_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayStateAvailable),
	}, string(p))

}

func (p TransitGatewayState_Process) isBeforeDestroyState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayStatePending),
		string(types.TransitGatewayStateAvailable),
		string(types.TransitGatewayStateModifying),
	}, string(p))

}

func (p TransitGatewayState_Process) isAfterDestroyState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayStatePending),
		string(types.TransitGatewayStateAvailable),
		string(types.TransitGatewayStateModifying),
		string(types.TransitGatewayStateDeleting),
	}, string(p))
}

func (p TransitGatewayState_Process) isOKState() bool {
	return p.isBeforeCreateState()
}

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

func (d *TransitGatewaysInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.TransitGateway).TransitGatewayArn, nil
	})
}

/******************************************************************************/
type BaseTransitGateway struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseTransitGateway) init(ctx context.Context, mode ReadResourceMode) error {
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

	if err := b.readResources(mode); err != nil {
		return err
	}

	return nil
}

func (b *BaseTransitGateway) readResources(mode ReadResourceMode) error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	// TODO: Replace if necessary
	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeTransitGateways(context.TODO(), &ec2.DescribeTransitGatewaysInput{
		Filters: *filters,
	})
	if err != nil {
		return err
	}

	for _, transitGateway := range resp.TransitGateways {
		_state := TransitGatewayState_Process(transitGateway.State)
		if _state.isState(mode) == true {
			b.ResourceData.Append(transitGateway)
		}
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
	if err := c.init(ctx, ReadResourceModeAfterCreate); err != nil { // ClusterName/ClusterType and client initialization
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
				{
					ResourceType: types.ResourceTypeTransitGateway,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}

		if err := c.waitUntilResouceAvailable(0, 0, 1, func() error {
			return c.readResources(ReadResourceModeAfterCreate)
		}); err != nil {
			return err
		}
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
	c.init(ctx, ReadResourceModeBeforeDestroy) // ClusterName/ClusterType and client initialization

	_data := c.ResourceData.GetData()
	for _, transitGateway := range _data {
		_entry := transitGateway.(types.TransitGateway)
		if _, err := c.client.DeleteTransitGateway(context.TODO(), &ec2.DeleteTransitGatewayInput{
			TransitGatewayId: _entry.TransitGatewayId,
		}); err != nil {
			return err
		}

	}

	if err := c.waitUntilResouceDestroy(0, 0, func() error {
		return c.readResources(ReadResourceModeAfterDestroy)
	}); err != nil {
		return err
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
	c.init(ctx, ReadResourceModeCommon) // ClusterName/ClusterType and client initialization
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
