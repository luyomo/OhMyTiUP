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
	"encoding/json"
	"fmt"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

// Mkdir is used to create directory on the target host
type CreateRouteTable struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
	isPrivate      bool `default:false`
}

// Execute implements the Task interface
func (c *CreateRouteTable) Execute(ctx context.Context) error {
	if c.isPrivate == true {
		c.createPrivateSubnets(*c.pexecutor, ctx)
	} else {
		c.createPublicSubnets(*c.pexecutor, ctx)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTable) String() string {
	return fmt.Sprintf("Echo: Creating route table")
}

func (c *CreateRouteTable) createPrivateSubnets(executor ctxt.Executor, ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", clusterName, clusterType, c.subClusterType), false)
	if err != nil {
		return nil
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return nil
	}

	zap.L().Debug("Print the route tables", zap.String("routeTables", routeTables.String()))
	if len(routeTables.RouteTables) > 0 {
		c.clusterInfo.privateRouteTableId = routeTables.RouteTables[0].RouteTableId
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-route-table --vpc-id %s --tag-specifications \"ResourceType=route-table,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=private}]\"", c.clusterInfo.vpcInfo.VpcId, clusterName, clusterType, c.subClusterType)
	zap.L().Debug("create-route-table", zap.String("command", command))
	var retRouteTable ResultRouteTable
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	if err = json.Unmarshal(stdout, &retRouteTable); err != nil {
		zap.L().Error("Failed to parse the json", zap.String("return route table", string(stdout)))
		return nil
	}

	zap.L().Debug("Print the variable", zap.String("route table id", retRouteTable.TheRouteTable.RouteTableId))
	c.clusterInfo.privateRouteTableId = retRouteTable.TheRouteTable.RouteTableId

	return nil
}

func (c *CreateRouteTable) createPublicSubnets(executor ctxt.Executor, ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", clusterName, clusterType, c.subClusterType), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	fmt.Printf("*** *** *** The parsed data is \n %#v \n\n\n", routeTables)
	if len(routeTables.RouteTables) > 0 {
		c.clusterInfo.publicRouteTableId = routeTables.RouteTables[0].RouteTableId
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-route-table --vpc-id %s --tag-specifications \"ResourceType=route-table,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=public}]\"", c.clusterInfo.vpcInfo.VpcId, clusterName, clusterType, c.subClusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	var retRouteTable ResultRouteTable
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	//	fmt.Printf("The output from the route table preparation <%s> \n\n\n", stdout)

	if err = json.Unmarshal(stdout, &retRouteTable); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n\n", err)
		return nil
	}
	//fmt.Printf("The stdout from the subnett preparation: %s \n\n\n", sub_stdout)
	fmt.Printf("The stdout from the subnett preparation: %s \n\n\n", retRouteTable.TheRouteTable.RouteTableId)
	c.clusterInfo.publicRouteTableId = retRouteTable.TheRouteTable.RouteTableId

	return nil
}

/******************************************************************************/

// Mkdir is used to create directory on the target host
type DestroyRouteTable struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyRouteTable) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	stdout, _, err := (*c.pexecutor).Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", clusterName, clusterType, c.subClusterType), false)
	if err != nil {
		return err
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return err
	}

	for _, routeTable := range routeTables.RouteTables {
		command := fmt.Sprintf("aws ec2 delete-route-table --route-table-id %s", routeTable.RouteTableId)
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
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
	return fmt.Sprintf("Echo: Destroying route table ")
}

/******************************************************************************/

// Mkdir is used to create directory on the target host
type ListRouteTable struct {
	pexecutor        *ctxt.Executor
	tableRouteTables *[][]string
}

// Execute implements the Task interface
func (c *ListRouteTable) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	stdout, _, err := (*c.pexecutor).Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" ", clusterName, clusterType), false)
	if err != nil {
		return err
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return err
	}

	for _, routeTable := range routeTables.RouteTables {
		componentName := "-"
		for _, tagItem := range routeTable.Tags {
			if tagItem.Key == "Type" {
				componentName = tagItem.Value
			}
		}
		for _, route := range routeTable.Routes {
			(*c.tableRouteTables) = append(*c.tableRouteTables, []string{
				componentName,
				routeTable.RouteTableId,
				route.DestinationCidrBlock,
				route.TransitGatewayId,
				route.GatewayId,
				route.State,
				route.Origin,
			})
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListRouteTable) String() string {
	return fmt.Sprintf("Echo: Listing route table ")
}
