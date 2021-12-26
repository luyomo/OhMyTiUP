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
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
	"strings"
	//	"time"
)

type VPCPeeringStatus struct {
	Code    string `json:"Code"`
	Message string `json:""Message`
}

type AccepterVpcInfo struct {
	VpcId string `json:"VpcId"`
}

type RequesterVpcInfo struct {
	CidrBlock string `json:"CidrBlock"`
	VpcId     string `json:"VpcId"`
}

type VPCPeeringConnection struct {
	Status                 VPCPeeringStatus `json:"Status"`
	RequesterVpcInfo       RequesterVpcInfo `json:"RequesterVpcInfo"`
	AccepterVpcInfo        AccepterVpcInfo  `json:"AccepterVpcInfo"`
	VpcPeeringConnectionId string           `json:"VpcPeeringConnectionId"`
}

type VPCPeeringConnections struct {
	VpcPeeringConnections []VPCPeeringConnection `json:"VpcPeeringConnections"`
}

type AcceptVPCPeering struct {
	pexecutor   *ctxt.Executor
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *AcceptVPCPeering) Execute(ctx context.Context) error {

	vpcs, err := getVPCInfos(*c.pexecutor, ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType})
	if err != nil {
		return err
	}

	var arrVpcs []string
	for _, vpc := range (*vpcs).Vpcs {
		arrVpcs = append(arrVpcs, vpc.VpcId)
	}

	command := fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=accepter-vpc-info.vpc-id,Values=%s\" ", strings.Join(arrVpcs, ","))
	fmt.Printf("The command is <%s> \n\n\n", command)

	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var vpcPeerings VPCPeeringConnections
	if err := json.Unmarshal(stdout, &vpcPeerings); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil
	}

	for _, vpcPeering := range vpcPeerings.VpcPeeringConnections {
		fmt.Printf("The vpc info is <%#v> \n\n\n", vpcPeering)
		if vpcPeering.Status.Code == "pending-acceptance" {
			command = fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s", vpcPeering.VpcPeeringConnectionId)
			fmt.Printf("The command is <%s> \n\n\n", command)

			_, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error is <%s> \n\n\n", string(stderr))
				return nil
			}

			routeTable, err := getRouteTableByVPC(*c.pexecutor, ctx, c.clusterName, vpcPeering.AccepterVpcInfo.VpcId)
			if err != nil {
				return err
			}

			command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", routeTable.RouteTableId, vpcPeering.RequesterVpcInfo.CidrBlock, vpcPeering.VpcPeeringConnectionId)
			fmt.Printf("The comamnd is <%s> \n\n\n", command)
			_, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return err
			}
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *AcceptVPCPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *AcceptVPCPeering) String() string {
	return fmt.Sprintf("Echo: Accepting VPC Peering")
}

/******************************************************************************/

type DestroyVpcPeering struct {
	pexecutor   *ctxt.Executor
	clusterName string
	clusterType string
}

func (c *DestroyVpcPeering) Execute(ctx context.Context) error {
	vpcs, err := getVPCInfos(*(c.pexecutor), ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType})
	if err != nil {
		return err
	}

	var arrVpcs []string
	for _, vpc := range (*vpcs).Vpcs {
		arrVpcs = append(arrVpcs, vpc.VpcId)
	}

	command := fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=accepter-vpc-info.vpc-id,Values=%s\" ", strings.Join(arrVpcs, ","))
	fmt.Printf("The command is <%s> \n\n\n", command)

	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var vpcPeerings VPCPeeringConnections
	if err := json.Unmarshal(stdout, &vpcPeerings); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil
	}

	for _, pcx := range vpcPeerings.VpcPeeringConnections {
		fmt.Printf("The vpc info is <%#v> \n\n\n", pcx)
		command := fmt.Sprintf("aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id %s", pcx.VpcPeeringConnectionId)
		_, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("ERRORS: delete-vpc-peering-connection  <%s> \n\n\n", string(stderr))
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyVpcPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVpcPeering) String() string {
	return fmt.Sprintf("Echo: Destroying vpc peering ")
}
