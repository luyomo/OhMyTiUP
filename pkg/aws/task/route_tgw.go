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
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type CreateRouteTgw struct {
	pexecutor       *ctxt.Executor
	subClusterType  string
	subClusterTypes []string
	client          *ec2.Client
}

// Execute implements the Task interface
func (c *CreateRouteTgw) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
	c.client = ec2.NewFromConfig(cfg)

	sourceVpcInfo, err := getVPCInfo(*c.pexecutor, ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType, subClusterType: c.subClusterType})

	if err != nil {
		if err.Error() == "No VPC found" {
			return nil
		}
		return err
	}
	if sourceVpcInfo == nil {
		return nil
	}

	routeTable, err := getRouteTable(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
	if err != nil {
		return err
	}

	transitGateway, err := getTransitGateway(*c.pexecutor, ctx, clusterName, clusterType)
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return errors.New("No transit gateway found")
	}

	sourceRouteTables, err := c.FetchRouteTables(clusterName, clusterType, c.subClusterType)
	if err != nil {
		return err
	}

	for _, targetSubClusterType := range c.subClusterTypes {
		vpcInfo, err := getVPCInfo(*c.pexecutor, ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType, subClusterType: targetSubClusterType})
		if err != nil {
			if err.Error() == "No VPC found" {
				continue
			}
			return err
		}
		if vpcInfo == nil {
			continue
		}

		routeHasExisted, err := c.RouteHasExists(sourceRouteTables, routeTable.RouteTableId, (*vpcInfo).CidrBlock, transitGateway.TransitGatewayId)
		if err != nil {
			return err
		}
		if routeHasExisted == false {

			command := fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --transit-gateway-id %s", routeTable.RouteTableId, (*vpcInfo).CidrBlock, transitGateway.TransitGatewayId)
			_, _, err = (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				return err
			}
		}

		targetRouteTable, err := getRouteTable(*c.pexecutor, ctx, clusterName, clusterType, targetSubClusterType)
		if err != nil {
			return err
		}

		targetRouteTables, err := c.FetchRouteTables(clusterName, clusterType, targetSubClusterType)
		if err != nil {
			return err
		}

		routeHasExisted, err = c.RouteHasExists(targetRouteTables, targetRouteTable.RouteTableId, (*sourceVpcInfo).CidrBlock, transitGateway.TransitGatewayId)
		if err != nil {
			return err
		}
		if routeHasExisted == false {
			command := fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block %s --transit-gateway-id %s", targetRouteTable.RouteTableId, (*sourceVpcInfo).CidrBlock, transitGateway.TransitGatewayId)

			_, _, err = (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTgw) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTgw) String() string {
	return fmt.Sprintf("Echo: Creating route tgw ")
}

// If the transit gateway is removed, need to check the status and remove to re-add it.
func (c *CreateRouteTgw) RouteHasExists(routeTables *[]types.RouteTable, routeTableId, cidr, transitGatewayId string) (bool, error) {
	for _, routeTable := range *routeTables {
		for _, route := range routeTable.Routes {
			if route.TransitGatewayId != nil {

				if routeTableId == *routeTable.RouteTableId && cidr == *route.DestinationCidrBlock {
					if route.State == "blackhole" {

						_, err := c.client.DeleteRoute(context.TODO(), &ec2.DeleteRouteInput{RouteTableId: aws.String(routeTableId), DestinationCidrBlock: aws.String(cidr)})
						if err != nil {
							return false, err
						}
						return false, nil
					}

					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (c *CreateRouteTgw) FetchRouteTables(clusterName, clusterType, subClusterType string) (*[]types.RouteTable, error) {

	var filters []types.Filter
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{clusterName}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{clusterType}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{subClusterType}})

	describeRouteTables, err := c.client.DescribeRouteTables(context.TODO(), &ec2.DescribeRouteTablesInput{Filters: filters})
	if err != nil {
		return nil, err
	}
	return &describeRouteTables.RouteTables, nil

}
