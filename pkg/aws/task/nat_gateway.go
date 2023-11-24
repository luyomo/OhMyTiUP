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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

/******************************************************************************/
func (b *Builder) CreateNATGateway(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateNATGateway{BaseNATGateway: BaseNATGateway{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypeNAT}}})
	return b
}

func (b *Builder) ListNATGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListNATGateway{
		BaseNATGateway: BaseNATGateway{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

func (b *Builder) DestroyNATGateway(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyNATGateway{BaseNATGateway: BaseNATGateway{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}}})
	return b
}

type NatGatewayState_Process types.NatGatewayState

func (p NatGatewayState_Process) isState(mode ReadResourceMode) bool {
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

func (p NatGatewayState_Process) isBeforeCreateState() bool {
	return ListContainElement([]string{
		string(types.NatGatewayStatePending),
		string(types.NatGatewayStateFailed),
	}, string(p))

}

func (p NatGatewayState_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string(types.NatGatewayStateAvailable),
	}, string(p))

}

func (p NatGatewayState_Process) isBeforeDestroyState() bool {
	return ListContainElement([]string{
		string(types.NatGatewayStatePending),
		string(types.NatGatewayStateFailed),
		string(types.NatGatewayStateAvailable),
	}, string(p))

}

func (p NatGatewayState_Process) isAfterDestroyState() bool {
	return ListContainElement([]string{
		string(types.NatGatewayStatePending),
		string(types.NatGatewayStateFailed),
		string(types.NatGatewayStateAvailable),
		string(types.NatGatewayStateDeleting),
	}, string(p))
}

func (p NatGatewayState_Process) isOKState() bool {
	return p.isBeforeCreateState()
}

/******************************************************************************/

type NATGateways struct {
	BaseResourceInfo
}

func (d *NATGateways) ToPrintTable() *[][]string {
	tableNATGateway := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(NATGateway)
		// tableNATGateway = append(tableNATGateway, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableNATGateway
}

func (d *NATGateways) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.NatGateway).NatGatewayId, nil
	})
}

/******************************************************************************/
type BaseNATGateway struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseNATGateway) init(ctx context.Context, mode ReadResourceMode) error {
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
		b.ResourceData = &NATGateways{}
	}

	if err := b.readResources(mode); err != nil {
		return err
	}

	return nil
}

func (b *BaseNATGateway) readResources(mode ReadResourceMode) error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	// Pattern02: Descibe using filters
	resp, err := b.client.DescribeNatGateways(context.TODO(), &ec2.DescribeNatGatewaysInput{Filter: *filters})
	if err != nil {
		return err
	}

	for _, natGateway := range resp.NatGateways {
		_state := NatGatewayState_Process(natGateway.State)
		if _state.isState(mode) == true {
			b.ResourceData.Append(natGateway)
		}
	}

	return nil
}

/******************************************************************************/
type CreateNATGateway struct {
	BaseNATGateway
}

// Execute implements the Task interface
func (c *CreateNATGateway) Execute(ctx context.Context) error {
	if err := c.init(ctx, ReadResourceModeAfterCreate); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		clusterSubnets, err := c.GetSubnetsInfo(1)
		if err != nil {
			return err
		}

		elasticAddress, err := c.GetElasticAddress(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}

		tags := c.MakeEC2Tags()

		if _, err = c.client.CreateNatGateway(context.TODO(), &ec2.CreateNatGatewayInput{
			SubnetId:     aws.String((*clusterSubnets)[0]),
			AllocationId: elasticAddress,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeNatgateway,
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

	if err := c.addRouteFromPrivate2NAT(); err != nil {
		return err
	}

	if err := c.CreateInternetGatewayRoute(); err != nil {
		return err
	}

	return nil
}

func (c *CreateNATGateway) addRouteFromPrivate2NAT() error {

	natGatewayId, err := c.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	listRouteTable := &ListRouteTable{BaseRouteTable: BaseRouteTable{BaseTask: BaseTask{
		pexecutor:      c.pexecutor,
		clusterName:    c.clusterName,
		clusterType:    c.clusterType,
		subClusterType: c.subClusterType,
		scope:          NetworkTypePrivate, // Used to search routes from private network rather than nat.
	}}}
	if err := listRouteTable.Execute(nil); err != nil {
		return err
	}

	// Need
	routeTable, err := listRouteTable.GetRouteTable()
	if err != nil {
		return err
	}

	for _, route := range routeTable.Routes {
		if route.NatGatewayId != nil && *route.NatGatewayId == *natGatewayId {
			return nil
		}
	}

	routeTableId, err := listRouteTable.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	if _, err := c.client.CreateRoute(context.TODO(), &ec2.CreateRouteInput{
		RouteTableId:         routeTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		NatGatewayId:         natGatewayId,
	}); err != nil {
		fmt.Printf("Failed when creating route \n\n\n\n\n\n")
		return err
	}

	return nil
}

func (c *CreateNATGateway) CreateInternetGatewayRoute() error {
	internetGatewayId, _, err := c.GetInternetGatewayID(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	routeTable, err := c.GetRouteTable()
	if err != nil {
		return err
	}

	for _, route := range routeTable.Routes {
		if route.NetworkInterfaceId != nil && *route.NetworkInterfaceId == *internetGatewayId {
			return nil
		}
	}

	if _, err := c.client.CreateRoute(context.TODO(), &ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            internetGatewayId,
	}); err != nil {
		fmt.Printf("Error: -------------- \n\n\n\n\n\n")
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateNATGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNATGateway) String() string {
	return fmt.Sprintf("Echo: Create NATGateway ... ...  ")
}

type DestroyNATGateway struct {
	BaseNATGateway
}

// Execute implements the Task interface
func (c *DestroyNATGateway) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeBeforeDestroy) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyNATGateway ****** \n\n\n")

	// clusterExistFlag, err := c.ResourceData.ResourceExist()
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("Starting to destoy<%#v> \n\n\n\n\n\n", clusterExistFlag)

	_id, err := c.ResourceData.GetResourceArn(ContinueIfNotExists)
	if err != nil {
		return err
	}

	if _id != nil {

		fmt.Printf("Starting to destoy <%s> \n\n\n\n\n\n", *_id)
		if _, err = c.client.DeleteNatGateway(context.TODO(), &ec2.DeleteNatGatewayInput{
			NatGatewayId: _id,
		}); err != nil {
			return err
		}

		if err := c.waitUntilResouceDestroy(0, 0, func() error {
			return c.readResources(ReadResourceModeAfterDestroy)
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyNATGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyNATGateway) String() string {
	return fmt.Sprintf("Echo: Destroying NATGateway")
}

type ListNATGateway struct {
	BaseNATGateway
}

// Execute implements the Task interface
func (c *ListNATGateway) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeCommon) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListNATGateway ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListNATGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListNATGateway) String() string {
	return fmt.Sprintf("Echo: List  ")
}
