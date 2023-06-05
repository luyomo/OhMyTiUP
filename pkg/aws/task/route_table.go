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
func (b *Builder) CreateRouteTable(pexecutor *ctxt.Executor, subClusterType string, network NetworkType) *Builder {

	b.tasks = append(b.tasks, &CreateRouteTable{
		BaseRouteTable: BaseRouteTable{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}},
	})

	return b
}

func (b *Builder) ListRouteTable(pexecutor *ctxt.Executor, tableRouteTables *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListRouteTable{
		BaseRouteTable:   BaseRouteTable{BaseTask: BaseTask{pexecutor: pexecutor}},
		tableRouteTables: tableRouteTables,
	})
	return b
}

func (b *Builder) DestroyRouteTable(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRouteTable{
		BaseRouteTable: BaseRouteTable{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}},
	})
	return b
}

/******************************************************************************/

type RouteTablesInfo struct {
	BaseResourceInfo
}

func (d *RouteTablesInfo) ToPrintTable() *[][]string {
	tableRouteTable := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(RouteTable)
		// tableRouteTable = append(tableRouteTable, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableRouteTable
}

func (d *RouteTablesInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.RouteTable).RouteTableId, nil
	})
}

func (d *RouteTablesInfo) GetRouteTableId() (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource(route table id) found ")
	}

	return (d.Data[0]).(*types.RouteTable).RouteTableId, nil

}

/******************************************************************************/
type BaseRouteTable struct {
	BaseTask

	ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseRouteTable) init(ctx context.Context) error {
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
		b.ResourceData = &RouteTablesInfo{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseRouteTable) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeRouteTables(context.TODO(), &ec2.DescribeRouteTablesInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, routeTable := range resp.RouteTables {
		b.ResourceData.Append(routeTable)
	}
	return nil
}

/******************************************************************************/
type CreateRouteTable struct {
	BaseRouteTable
}

// Execute implements the Task interface
func (c *CreateRouteTable) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	// Check resource's existness
	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	// Skip the route table preparation if it exists
	if clusterExistFlag == false {
		tags := c.MakeEC2Tags()

		vpcId, err := c.GetVpcItem("VpcId")
		if err != nil {
			return err
		}

		if _, err = c.client.CreateRouteTable(context.TODO(), &ec2.CreateRouteTableInput{
			VpcId: vpcId,
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeRouteTable,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}

		if err := c.readResources(); err != nil {
			return err
		}

		// TODO: Check cluster status until expected status
	}

	// Process as below:
	// 01. Create the internet gateway if it does not exit.
	// 02. Attach the internet gateway to VPC
	// 03. Create route for the route table

	if c.scope == NetworkTypePublic {

		if err := c.CreateInternetGateway(); err != nil {
			return err
		}

		if err := c.AttachGW2VPC(); err != nil {
			return err
		}

		if err := c.CreateInternetGatewayRoute(); err != nil {
			return err
		}

	}

	// Process as below for nat:
	// 01. Create internet gateway
	if c.scope == NetworkTypeNAT {
		if err := c.CreateInternetGateway(); err != nil {
			return err
		}

		if err := c.AttachGW2VPC(); err != nil {
			return err
		}

		// Create NAT

		// if err := c.CreateRoute(); err != nil {
		// 	return err
		// }
	}

	return nil
}

func (c *CreateRouteTable) CreateInternetGateway() error {
	filters := c.MakeEC2Filters()

	resp, err := c.client.DescribeInternetGateways(context.TODO(), &ec2.DescribeInternetGatewaysInput{Filters: *filters})
	if err != nil {
		return err
	}

	tags := c.MakeEC2Tags()
	if len(resp.InternetGateways) == 0 {
		if _, err := c.client.CreateInternetGateway(context.TODO(), &ec2.CreateInternetGatewayInput{
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeInternetGateway,
					Tags:         *tags,
				},
			}}); err != nil {
			return err
		}
	}

	return nil
}

func (c *CreateRouteTable) AttachGW2VPC() error {
	internetGatewayId, hasAttached, err := c.GetInternetGatewayId(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	if hasAttached == true {
		return nil
	}

	vpcId, err := c.GetVpcItem("VpcId")
	if err != nil {
		return err
	}

	if _, err := c.client.AttachInternetGateway(context.TODO(), &ec2.AttachInternetGatewayInput{
		InternetGatewayId: internetGatewayId,
		VpcId:             vpcId,
	}); err != nil {
		return err
	}

	return nil
}

func (c *CreateRouteTable) CreateInternetGatewayRoute() error {
	internetGatewayId, _, err := c.GetInternetGatewayId(ThrowErrorIfNotExists)
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
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTable) String() string {
	return fmt.Sprintf("Echo: Create RouteTable ... ...  ")
}

type DestroyRouteTable struct {
	BaseRouteTable
}

// Execute implements the Task interface
func (c *DestroyRouteTable) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyRouteTable ****** \n\n\n")

	_data := c.ResourceData.GetData()
	for _, routeTable := range _data {
		_entry := routeTable.(types.RouteTable)
		if _, err := c.client.DeleteRouteTable(context.TODO(), &ec2.DeleteRouteTableInput{
			RouteTableId: _entry.RouteTableId,
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyRouteTable) String() string {
	return fmt.Sprintf("Echo: Destroying RouteTable")
}

type ListRouteTable struct {
	BaseRouteTable

	tableRouteTables *[][]string
}

// Execute implements the Task interface
func (c *ListRouteTable) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListRouteTable ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListRouteTable) String() string {
	return fmt.Sprintf("Echo: List  ")
}
