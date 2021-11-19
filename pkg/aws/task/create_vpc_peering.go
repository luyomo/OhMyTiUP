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
	"time"

	"github.com/luyomo/tisample/pkg/aws/executor"
	"github.com/luyomo/tisample/pkg/aws/spec"
)

type VPCStatus struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type VpcPeer struct {
	VpcPeeringConnectionId string    `json:"VpcPeeringConnectionId"`
	VpcStatus              VPCStatus `json:"Status"`
}

type VpcConnection struct {
	VpcPeeringConnection VpcPeer `json:"VpcPeeringConnection"`
}

type VpcConnections struct {
	VpcPeeringConnections []VpcPeer `json:"VpcPeeringConnections"`
}

type CreateVpcPeering struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *CreateVpcPeering) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	fmt.Printf("The aurora vpc name is <%#v>\n\n\n", c.awsTopoConfigs.Aurora)

	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", c.awsTopoConfigs.Aurora.Name), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	var vpcs Vpcs
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The vpsc from aurora is <%s>\n\n\n", vpcs)

	if len(vpcs.Vpcs) == 0 {
		fmt.Printf("There is no matched aurora vpc")
		return nil
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var vpcConnections VpcConnections
	if err = json.Unmarshal(stdout, &vpcConnections); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}
	state := ""
	for _, pcx := range vpcConnections.VpcPeeringConnections {
		fmt.Printf("The pcx is <%#v> \n\n\n", pcx)
		if pcx.VpcStatus.Code == "active" {
			state = "active"
			clusterInfo.pcxTidb2Aurora = pcx.VpcPeeringConnectionId
		}
	}
	fmt.Printf("The vpc state is <%s> and <%s> \n\n\n", state, clusterInfo.pcxTidb2Aurora)
	//	fmt.Printf("The pcx connection from aurora is <%#v>\n\n\n", vpcConnections)

	if state == "" {

		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc-peering-connection --vpc-id %s --peer-vpc-id  %s --tag-specification \"ResourceType=vpc-peering-connection,Tags=[{Key=Name,Value=%s}]\"", vpcs.Vpcs[0].VpcId, clusterInfo.vpcInfo.VpcId, c.clusterName), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
		fmt.Printf("The vpc peering is <%s> \n\n\n", stdout)
		var vpcConnection VpcConnection
		if err = json.Unmarshal(stdout, &vpcConnection); err != nil {
			fmt.Printf("The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The parsed data is %#v \n\n\n", vpcConnection)
		clusterInfo.pcxTidb2Aurora = vpcConnection.VpcPeeringConnection.VpcPeeringConnectionId

		time.Sleep(5 * time.Second)

		fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s ", clusterInfo.pcxTidb2Aurora), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
		fmt.Printf("The output data is <%s> \n\n\r\r", stdout)
	}

	if state == "pending-acceptance" {
		fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s ", clusterInfo.pcxTidb2Aurora), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
		fmt.Printf("The output data is <%s> \n\n\r\r", stdout)
	}

	// Add route table for the pcs
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", clusterInfo.publicRouteTableId, vpcs.Vpcs[0].CidrBlock, clusterInfo.pcxTidb2Aurora), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", clusterInfo.privateRouteTableId, vpcs.Vpcs[0].CidrBlock, clusterInfo.pcxTidb2Aurora), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=aurora\"", c.awsTopoConfigs.Aurora.Name), false)
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
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", routeTables.RouteTables[0].RouteTableId, clusterInfo.vpcInfo.CidrBlock, clusterInfo.pcxTidb2Aurora), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateVpcPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateVpcPeering) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
