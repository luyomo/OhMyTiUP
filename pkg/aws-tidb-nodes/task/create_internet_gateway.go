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
	//	"github.com/luyomo/tisample/pkg/workstation/ctxt"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
	//"strconv"
	//"strings"
	//"time"
)

type InternetGateway struct {
	InternetGatewayId string `json:"InternetGatewayId"`
}

type InternetGateways struct {
	InternetGateways []InternetGateway `json:"InternetGateways"`
}

type NewInternetGateway struct {
	InternetGateway InternetGateway `json:"InternetGateway"`
}

// Mkdir is used to create directory on the target host
type CreateInternetGateway struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreateInternetGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The type of local is <%T> \n\n\n", local)
	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-internet-gateways --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	//fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	var internetGateways InternetGateways
	if err = json.Unmarshal(stdout, &internetGateways); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	if len(internetGateways.InternetGateways) > 0 {
		fmt.Printf("*** *** *** Got the internet gateways <%#v> \n\n\n", internetGateways)
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-internet-gateway --tag-specifications \"ResourceType=internet-gateway,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb}]\"", c.clusterName)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	var newInternetGateway NewInternetGateway
	if err = json.Unmarshal(stdout, &newInternetGateway); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n\n", err)
		return nil
	}
	//	fmt.Printf("The stdout from the internet gateway preparation: %s \n\n\n", stdout)
	fmt.Printf("The stdout from the internet gateway preparation: %#v \n\n\n", newInternetGateway)
	//fmt.Printf("The stdout from the subnett preparation: %s and %s \n\n\n", newSubnet.Subnet.State, newSubnet.Subnet.CidrBlock)
	//	associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.routeTableId, local, ctx)
	//	clusterInfo.subnets = append(clusterInfo.subnets, newSubnet.Subnet.SubnetId)
	command = fmt.Sprintf("aws ec2 attach-internet-gateway --internet-gateway-id %s --vpc-id %s", newInternetGateway.InternetGateway.InternetGatewayId, clusterInfo.vpcInfo.VpcId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out is <%s> \n\n\n", stdout)

	command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block 0.0.0.0/0 --gateway-id %s", clusterInfo.publicRouteTableId, newInternetGateway.InternetGateway.InternetGatewayId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out is <%s> \n\n\n", stdout)

	//

	return nil
}

// Rollback implements the Task interface
func (c *CreateInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateInternetGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
