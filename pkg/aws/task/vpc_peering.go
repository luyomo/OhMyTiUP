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
	// "encoding/json"
	"fmt"
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"github.com/luyomo/tisample/pkg/aws/spec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "go.uber.org/zap"
	// "strings"
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

// type VPCPeeringConnection struct {
// 	Status                 VPCPeeringStatus `json:"Status"`
// 	RequesterVpcInfo       RequesterVpcInfo `json:"RequesterVpcInfo"`
// 	AccepterVpcInfo        AccepterVpcInfo  `json:"AccepterVpcInfo"`
// 	VpcPeeringConnectionId string           `json:"VpcPeeringConnectionId"`
// }

// type VPCPeeringConnections struct {
// 	VpcPeeringConnections []VPCPeeringConnection `json:"VpcPeeringConnections"`
// }

type VPCPeeringConnection struct {
	VpcConnectionId  string
	Status           string
	RequesterVpcId   string
	RequesterVpcCIDR string
	AcceptorVpcId    string
	AcceptorVpcCIDR  string
	AcceptorVpcName  string
}

// type VPCPeeringConnections struct {
// 	VpcPeeringConnections []VPCPeeringConnection
// }

type AcceptVPCPeering struct {
	pexecutor *ctxt.Executor
}

// Execute implements the Task interface
func (c *AcceptVPCPeering) Execute(ctx context.Context) error {
	vpcPeeringConnections, err := searchVPCPeering(ctx, []string{"dm", "workstation"})
	if err != nil {
		return err
	}

	for _, vpcPeeringConnection := range *vpcPeeringConnections {
		fmt.Printf("The data is <%#v> \n\n\n", vpcPeeringConnection)
		// *(c.tableVpcPeeringInfo) = append(*(c.tableVpcPeeringInfo), []string{vpcPeeringConnection.VpcConnectionId,
		// 	vpcPeeringConnection.Status,
		// 	vpcPeeringConnection.RequesterVpcId,
		// 	vpcPeeringConnection.RequesterVpcCIDR,
		// 	vpcPeeringConnection.AcceptorVpcId + "/" + vpcPeeringConnection.AcceptorVpcName,
		// 	vpcPeeringConnection.AcceptorVpcCIDR,
		// })
	}

	// clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)

	// 01. Find out the vpc peering waiting for accept
	// 02. accept the vpc peering
	// 03. Wait until it becomes the valid
	// 04. Go back
	// vpcs, err := getVPCInfos(*c.pexecutor, ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType})
	// if err != nil {
	// 	return err
	// }

	// var arrVpcs []string
	// for _, vpc := range (*vpcs).Vpcs {
	// 	arrVpcs = append(arrVpcs, vpc.VpcId)
	// }

	// command := fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=accepter-vpc-info.vpc-id,Values=%s\" ", strings.Join(arrVpcs, ","))

	// stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	// if err != nil {
	// 	return nil
	// }
	// var vpcPeerings VPCPeeringConnections
	// if err := json.Unmarshal(stdout, &vpcPeerings); err != nil {
	// 	zap.L().Debug("The error to parse the string ", zap.Error(err))
	// 	return nil
	// }

	// for _, vpcPeering := range vpcPeerings.VpcPeeringConnections {
	// 	if vpcPeering.Status.Code == "pending-acceptance" {
	// 		command = fmt.Sprintf("aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id %s", vpcPeering.VpcPeeringConnectionId)

	// 		_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		routeTable, err := getRouteTableByVPC(*c.pexecutor, ctx, clusterName, vpcPeering.AccepterVpcInfo.VpcId)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --vpc-peering-connection-id %s", routeTable.RouteTableId, vpcPeering.RequesterVpcInfo.CidrBlock, vpcPeering.VpcPeeringConnectionId)
	// 		_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}

	// }

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
	pexecutor *ctxt.Executor
}

func (c *DestroyVpcPeering) Execute(ctx context.Context) error {
	// clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)

	return nil

	// vpcs, err := getVPCInfos(*(c.pexecutor), ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType})
	// if err != nil {
	// 	return err
	// }

	// var arrVpcs []string
	// for _, vpc := range (*vpcs).Vpcs {
	// 	arrVpcs = append(arrVpcs, vpc.VpcId)
	// }

	// command := fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=accepter-vpc-info.vpc-id,Values=%s\" ", strings.Join(arrVpcs, ","))
	// stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	// if err != nil {
	// 	return nil
	// }
	// var vpcPeerings VPCPeeringConnections
	// if err := json.Unmarshal(stdout, &vpcPeerings); err != nil {
	// 	zap.L().Debug("The error to parse the string ", zap.Error(err))
	// 	return nil
	// }

	// for _, pcx := range vpcPeerings.VpcPeeringConnections {

	// 	command := fmt.Sprintf("aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id %s", pcx.VpcPeeringConnectionId)
	// 	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// return nil
}

// Rollback implements the Task interface
func (c *DestroyVpcPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVpcPeering) String() string {
	return fmt.Sprintf("Echo: Destroying vpc peering ")
}

// ------
type ListVpcPeering struct {
	pexecutor           *ctxt.Executor
	subClusterTypes     []string
	tableVpcPeeringInfo *[][]string
}

// Execute implements the Task interface
func (c *ListVpcPeering) Execute(ctx context.Context) error {

	vpcPeeringConnections, err := searchVPCPeering(ctx, c.subClusterTypes)
	if err != nil {
		return err
	}

	for _, vpcPeeringConnection := range *vpcPeeringConnections {
		*(c.tableVpcPeeringInfo) = append(*(c.tableVpcPeeringInfo), []string{vpcPeeringConnection.VpcConnectionId,
			vpcPeeringConnection.Status,
			vpcPeeringConnection.RequesterVpcId,
			vpcPeeringConnection.RequesterVpcCIDR,
			vpcPeeringConnection.AcceptorVpcId + "/" + vpcPeeringConnection.AcceptorVpcName,
			vpcPeeringConnection.AcceptorVpcCIDR,
		})
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListVpcPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListVpcPeering) String() string {
	return fmt.Sprintf("Echo: Listing vpc peering ")
}

// ------------ ----------- ------------
func searchVPCPeering(ctx context.Context, subClusterTypes []string) (*[]VPCPeeringConnection, error) {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	var vpcPeeringConnections []VPCPeeringConnection

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Name"),
		Values: []string{clusterName},
	})

	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Cluster"),
		Values: []string{clusterType},
	})

	filters = append(filters, types.Filter{
		Name: aws.String("tag:Type"),
		// Values: []string{c.subClusterType},
		Values: subClusterTypes,
	})

	describeVpcsInput := &ec2.DescribeVpcsInput{Filters: filters}
	describeVpcs, err := client.DescribeVpcs(context.TODO(), describeVpcsInput)
	if err != nil {
		return nil, err
	}

	vpcId := ""
	for _, vpc := range describeVpcs.Vpcs {
		vpcName := ""
		vpcId = *(vpc.VpcId)
		// fmt.Printf("The vpcs info is <%#v> \n\n\n", vpc)
		for _, tag := range vpc.Tags {
			// fmt.Printf("The vpcs info is <%#v> \n\n\n", tag)
			if *(tag.Key) == "Type" {
				vpcName = *(tag.Value)
			}
		}

		var vpcPeeringFilters []types.Filter
		vpcPeeringFilters = append(vpcPeeringFilters, types.Filter{
			Name:   aws.String("accepter-vpc-info.vpc-id"),
			Values: []string{vpcId},
		})

		describeVpcPeeringConnectionsInput := &ec2.DescribeVpcPeeringConnectionsInput{Filters: vpcPeeringFilters}
		describeVpcPeeringConnections, err := client.DescribeVpcPeeringConnections(context.TODO(), describeVpcPeeringConnectionsInput)
		if err != nil {
			return nil, err
		}

		// fmt.Printf("The vpc peering is <%#v> \n\n\n", describeVpcPeeringConnections)
		for _, vpcPeering := range describeVpcPeeringConnections.VpcPeeringConnections {
			var vpcPeeringConnection VPCPeeringConnection

			vpcPeeringConnection.VpcConnectionId = *(vpcPeering.VpcPeeringConnectionId)
			vpcPeeringConnection.Status = string(vpcPeering.Status.Code)
			vpcPeeringConnection.RequesterVpcId = *(vpcPeering.RequesterVpcInfo.VpcId)
			vpcPeeringConnection.RequesterVpcCIDR = *(vpcPeering.RequesterVpcInfo.CidrBlock)
			vpcPeeringConnection.AcceptorVpcId = *(vpcPeering.AccepterVpcInfo.VpcId)
			vpcPeeringConnection.AcceptorVpcCIDR = *(vpc.CidrBlock)
			vpcPeeringConnection.AcceptorVpcName = vpcName

			vpcPeeringConnections = append(vpcPeeringConnections, vpcPeeringConnection)

			// // var vpcPeeringConnections VPCPeeringConnections
			// *(c.tableVpcPeeringInfo) = append(*(c.tableVpcPeeringInfo), []string{*(vpcPeering.VpcPeeringConnectionId),
			// 	string(vpcPeering.Status.Code),
			// 	*(vpcPeering.RequesterVpcInfo.VpcId),
			// 	*(vpcPeering.RequesterVpcInfo.CidrBlock),
			// 	*(vpcPeering.AccepterVpcInfo.VpcId) + "/" + vpcName,
			// 	*(vpc.CidrBlock),
			// })

		}
	}

	return &vpcPeeringConnections, nil
}
