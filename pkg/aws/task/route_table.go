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
func (b *Builder) CreateRouteTable(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	var scope string
	if isPrivate == true {
		scope = "private"
	} else {
		scope = "public"
	}
	b.tasks = append(b.tasks, &CreateRouteTable{
		BaseRouteTable: BaseRouteTable{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: scope}},
		clusterInfo:    clusterInfo,
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

func (d *RouteTablesInfo) GetResourceArn() (*string, error) {
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

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateRouteTable) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	fmt.Printf("CreateRouteTable -> ClusterName: %s, ClusterType: %s, subClusterType: %s, scope: %s \n\n\n\n", c.clusterName, c.clusterType, c.subClusterType, c.scope)

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

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

		// TODO: Check cluster status until expected status
	}

	// if c.subClusterType == "redshift" {
	// 	return nil
	// }
	fmt.Printf("ClusterName: %s, ClusterType: %s, subClusterType: %s, scope: %s \n\n\n\n", c.clusterName, c.clusterType, c.subClusterType, c.scope)
	if c.scope == "public" {
		fmt.Printf("------------------- \n\n\n\n\n")
		if err := c.CreateInternetGateway(); err != nil {
			return err
		}

		if err := c.AttachGW2VPC(); err != nil {
			return err
		}

		if err := c.CreateRoute(); err != nil {
			return err
		}

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
	fmt.Printf("The response is <%#v> \n\n\n\n\n", resp.InternetGateways)
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

func (c *CreateRouteTable) getInternetGatewayId() (*string, *string, error) {
	filters := c.MakeEC2Filters()
	resp, err := c.client.DescribeInternetGateways(context.TODO(), &ec2.DescribeInternetGatewaysInput{Filters: *filters})
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("The internete gateway is <%#v> \n\n\n\n", resp)
	if len(resp.InternetGateways) > 1 {
		return nil, nil, errors.New("Multiple internet gateways")
	}
	if len(resp.InternetGateways) == 0 {
		return nil, nil, errors.New("No internet gateways found")
	}

	internetGatewayId := *resp.InternetGateways[0].InternetGatewayId

	if len(resp.InternetGateways[0].Attachments) == 0 {
		return &internetGatewayId, nil, nil
	}

	attachedVpc := *resp.InternetGateways[0].Attachments[0].VpcId

	return &internetGatewayId, &attachedVpc, nil

}

func (c *CreateRouteTable) AttachGW2VPC() error {
	internetGatewayId, attachedVpc, err := c.getInternetGatewayId()
	if err != nil {
		return err
	}

	if attachedVpc != nil {
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
func (c *CreateRouteTable) CreateRoute() error {
	internetGatewayId, _, err := c.getInternetGatewayId()
	if err != nil {
		return err
	}

	fmt.Printf("The internet gateway is <%s> \n\n\n", internetGatewayId)

	var _routeTableId *string
	// _routes := c.ResourceData.GetData()
	for _, _entry := range c.ResourceData.GetData() {
		_routeTable := _entry.(types.RouteTable)
		_routeTableId = _routeTable.RouteTableId
		fmt.Printf("Route data: <%s> \n\n\n", *_routeTable.RouteTableId)
		for _, _route := range _routeTable.Routes {
			if _route.NetworkInterfaceId != nil && *_route.NetworkInterfaceId == *internetGatewayId {
				return nil
			}
			fmt.Printf("Route : <%s> and <%s> \n\n\n", *_route.DestinationCidrBlock, _route.NetworkInterfaceId)
		}

	}

	if _, err := c.client.CreateRoute(context.TODO(), &ec2.CreateRouteInput{
		RouteTableId:         _routeTableId,
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
	clusterInfo *ClusterInfo
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
