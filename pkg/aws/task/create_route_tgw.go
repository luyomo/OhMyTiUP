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
	//	"encoding/json"
	"fmt"
	//	"time"

	//	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/executor"
)

type CreateRouteTgw struct {
	user            string
	host            string
	clusterName     string
	clusterType     string
	subClusterType  string
	subClusterTypes []string
}

// Execute implements the Task interface
func (c *CreateRouteTgw) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	sourceVpcInfo, err := getVPC(local, ctx, c.clusterName, c.clusterType, c.subClusterType)
	if err != nil {
		return err
	}
	fmt.Printf("The source vpc info is <%s> \n\n\n", sourceVpcInfo.CidrBlock)

	routeTable, err := getRouteTable(local, ctx, c.clusterName, c.clusterType, c.subClusterType)
	if err != nil {
		return err
	}
	fmt.Printf("The route table is <%#v> \n\n\n", routeTable)

	transitGateway, err := getTransitGateway(local, ctx, c.clusterName)
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return errors.New("No transit gateway found")
	}

	fmt.Printf("The transit gateway is <%#v> \n\n\n", transitGateway)

	for _, targetSubClusterType := range c.subClusterTypes {
		fmt.Printf("The data is <%#v> \n\n\n", targetSubClusterType)
		vpcInfo, err := getVPC(local, ctx, c.clusterName, c.clusterType, targetSubClusterType)
		if err != nil {
			return err
		}
		fmt.Printf("The vpc info is <%s> \n\n\n", vpcInfo.CidrBlock)

		command := fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --transit-gateway-id %s", routeTable.RouteTableId, vpcInfo.CidrBlock, transitGateway.TransitGatewayId)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return err
		}
		fmt.Printf("The result from create-transit-gateway <%s> \n\n\n", string(stdout))

		targetRouteTable, err := getRouteTable(local, ctx, c.clusterName, c.clusterType, targetSubClusterType)
		if err != nil {
			return err
		}
		fmt.Printf("The target Routable id <%#v> \n\n\n", targetRouteTable)

		command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --transit-gateway-id %s", targetRouteTable.RouteTableId, sourceVpcInfo.CidrBlock, transitGateway.TransitGatewayId)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		stdout, stderr, err = local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return err
		}
		fmt.Printf("The result from create-transit-gateway <%s> \n\n\n", string(stdout))
	}

	////var sourceVPCInfo, targetVPCInfo Vpc
	//err = getVPC(local, ctx, c.sourceVPC, &sourceVPCInfo)
	//if err != nil {
	//	fmt.Printf("Failed to fetch the vpc info \n\n\n")
	//}
	//fmt.Printf("The source vpc info is <%#v> \n\n\n", sourceVPCInfo)

	/*
		//fmt.Printf("The aurora vpc name is <%#v>\n\n\n", c.awsTopoConfigs.Aurora)

		//	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", c.awsTopoConfigs.Aurora.Name), false)
		stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", "test"), false)
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
				c.clusterInfo.pcxTidb2Aurora = pcx.VpcPeeringConnectionId
			}
		}
		fmt.Printf("The vpc state is <%s> and <%s> \n\n\n", state, c.clusterInfo.pcxTidb2Aurora)
		//	fmt.Printf("The pcx connection from aurora is <%#v>\n\n\n", vpcConnections)

		if state == "" {

			stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc-peering-connection --vpc-id %s --peer-vpc-id  %s --tag-specification \"ResourceType=vpc-peering-connection,Tags=[{Key=Name,Value=%s}]\"", vpcs.Vpcs[0].VpcId, c.clusterInfo.vpcInfo.VpcId, c.clusterName), false)
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
			c.clusterInfo.pcxTidb2Aurora = vpcConnection.VpcPeeringConnection.VpcPeeringConnectionId

			time.Sleep(5 * time.Second)

			fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
			stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s ", c.clusterInfo.pcxTidb2Aurora), false)
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
			stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s ", c.clusterInfo.pcxTidb2Aurora), false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return nil
			}
			fmt.Printf("The output data is <%s> \n\n\r\r", stdout)
		}

		// Add route table for the pcs
		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", c.clusterInfo.publicRouteTableId, vpcs.Vpcs[0].CidrBlock, c.clusterInfo.pcxTidb2Aurora), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}

		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", c.clusterInfo.privateRouteTableId, vpcs.Vpcs[0].CidrBlock, c.clusterInfo.pcxTidb2Aurora), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}

		//	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=aurora\"", c.awsTopoConfigs.Aurora.Name), false)
		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=aurora\"", "test"), false)
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
		stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", routeTables.RouteTables[0].RouteTableId, c.clusterInfo.vpcInfo.CidrBlock, c.clusterInfo.pcxTidb2Aurora), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
	*/
	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTgw) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTgw) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
