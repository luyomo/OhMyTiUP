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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/ctxt"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
	"go.uber.org/zap"
	"strings"
)

type RouteTable struct {
	RouteTableId string `json:"RouteTableId"`
}

type ResultRouteTable struct {
	TheRouteTable RouteTable `json:"RouteTable"`
}

type RouteTables struct {
	RouteTables []RouteTable `json:"RouteTables"`
}

func (r RouteTable) String() string {
	return fmt.Sprintf("RouteTableId:%s", r.RouteTableId)
}

func (r ResultRouteTable) String() string {
	return fmt.Sprintf("RetRouteTable:%s", r.String())
}

func (r RouteTables) String() string {
	var res []string
	for _, route := range r.RouteTables {
		res = append(res, route.String())
	}
	return fmt.Sprintf("RouteTables:%s", strings.Join(res, ","))
}

// Mkdir is used to create directory on the target host
type CreateRouteTable struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreateRouteTable) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	c.createPrivateSubnets(local, ctx)

	c.createPublicSubnets(local, ctx)

	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTable) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func (c *CreateRouteTable) createPrivateSubnets(executor ctxt.Executor, ctx context.Context) error {
	// Get the available zones
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName), false)
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
		clusterInfo.privateRouteTableId = routeTables.RouteTables[0].RouteTableId
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-route-table --vpc-id %s --tag-specifications \"ResourceType=route-table,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=private}]\"", clusterInfo.vpcInfo.VpcId, c.clusterName)
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
	clusterInfo.privateRouteTableId = retRouteTable.TheRouteTable.RouteTableId

	return nil
}

func (c *CreateRouteTable) createPublicSubnets(executor ctxt.Executor, ctx context.Context) error {
	// Get the available zones
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName), false)
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
		clusterInfo.publicRouteTableId = routeTables.RouteTables[0].RouteTableId
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-route-table --vpc-id %s --tag-specifications \"ResourceType=route-table,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=public}]\"", clusterInfo.vpcInfo.VpcId, c.clusterName)
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
	clusterInfo.publicRouteTableId = retRouteTable.TheRouteTable.RouteTableId

	return nil
}
