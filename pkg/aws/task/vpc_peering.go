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
	pexecutor     *ctxt.Executor
	listComponent []string
}

// Execute implements the Task interface
func (c *AcceptVPCPeering) Execute(ctx context.Context) error {
	vpcPeeringConnections, err := searchVPCPeering(ctx, c.listComponent)
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	for _, vpcPeeringConnection := range *vpcPeeringConnections {
		// fmt.Printf("The data is <%#v> \n\n\n", vpcPeeringConnection)

		clusterName := ctx.Value("clusterName").(string)
		clusterType := ctx.Value("clusterType").(string)
		if vpcPeeringConnection.Status == "pending-acceptance" {

			acceptVpcPeeringConnectionInput := &ec2.AcceptVpcPeeringConnectionInput{VpcPeeringConnectionId: aws.String(vpcPeeringConnection.VpcConnectionId)}
			if _, err := client.AcceptVpcPeeringConnection(context.TODO(), acceptVpcPeeringConnectionInput); err != nil {
				return err
			}

			tags := []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(clusterName),
				},
				{
					Key:   aws.String("Type"),
					Value: aws.String(vpcPeeringConnection.AcceptorVpcName),
				},
			}

			createTagsInput := &ec2.CreateTagsInput{Tags: tags, Resources: []string{vpcPeeringConnection.VpcConnectionId}}
			if _, err := client.CreateTags(context.TODO(), createTagsInput); err != nil {
				return err
			}
		}

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
			Name:   aws.String("tag:Type"),
			Values: []string{vpcPeeringConnection.AcceptorVpcName},
		})

		describeRouteTablesInput := &ec2.DescribeRouteTablesInput{Filters: filters}
		describeRouteTables, err := client.DescribeRouteTables(context.TODO(), describeRouteTablesInput)
		if err != nil {
			return err
		}

		hasRegistered := false
		for _, routeTable := range describeRouteTables.RouteTables {
			for _, route := range routeTable.Routes {
				if vpcPeeringConnection.RequesterVpcCIDR == *route.DestinationCidrBlock {
					hasRegistered = true
				}
			}
		}
		if hasRegistered == false {
			createRouteInput := &ec2.CreateRouteInput{RouteTableId: describeRouteTables.RouteTables[0].RouteTableId, DestinationCidrBlock: &vpcPeeringConnection.RequesterVpcCIDR, VpcPeeringConnectionId: aws.String(vpcPeeringConnection.VpcConnectionId)}

			if _, err := client.CreateRoute(context.TODO(), createRouteInput); err != nil {
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
	pexecutor     *ctxt.Executor
	listComponent []string
}

func (c *DestroyVpcPeering) Execute(ctx context.Context) error {

	vpcPeeringConnections, err := searchVPCPeering(ctx, c.listComponent)
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	for _, vpcPeeringConnection := range *vpcPeeringConnections {
		if vpcPeeringConnection.Status == "active" {
			deleteVpcPeeringConnectionInput := &ec2.DeleteVpcPeeringConnectionInput{VpcPeeringConnectionId: aws.String(vpcPeeringConnection.VpcConnectionId)}
			if _, err := client.DeleteVpcPeeringConnection(context.TODO(), deleteVpcPeeringConnectionInput); err != nil {
				return err
			}
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
		Name:   aws.String("tag:Type"),
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
		for _, tag := range vpc.Tags {
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
			if vpcPeering.RequesterVpcInfo.CidrBlock != nil {
				vpcPeeringConnection.RequesterVpcCIDR = *(vpcPeering.RequesterVpcInfo.CidrBlock)
			}
			vpcPeeringConnection.AcceptorVpcId = *(vpcPeering.AccepterVpcInfo.VpcId)
			vpcPeeringConnection.AcceptorVpcCIDR = *(vpc.CidrBlock)
			vpcPeeringConnection.AcceptorVpcName = vpcName

			vpcPeeringConnections = append(vpcPeeringConnections, vpcPeeringConnection)

		}
	}

	return &vpcPeeringConnections, nil
}
