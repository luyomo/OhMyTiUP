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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	//	"strconv"
	//"strings"
	//"time"
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

// Mkdir is used to create directory on the target host
type CreateRouteTable struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateRouteTable) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisamplenodes\"", false)
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
		clusterInfo.routeTableId = routeTables.RouteTables[0].RouteTableId
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-route-table --vpc-id %s --tag-specifications \"ResourceType=route-table,Tags=[{Key=Name,Value=tisamplenodes}]\"", clusterInfo.vpcInfo.VpcId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	var retRouteTable ResultRouteTable
	stdout, stderr, err = local.Execute(ctx, command, false)
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

//func getNextCidr(cidr string, idx int) string {
//	ip := strings.Split(cidr, "/")[0]
//	ipSegs := strings.Split(ip, ".")
//	//	maskLen := strings.Split(cidr, "/")[1]
//	return ipSegs[0] + "." + ipSegs[1] + "." + strconv.Itoa(idx) + ".0/24"
//}
