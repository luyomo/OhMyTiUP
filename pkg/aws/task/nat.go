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
	"errors"
	"fmt"
	"time"
	// "sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/logger/log"
)

type CreateNAT struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *CreateNAT) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	vpc, err := getVPCInfo(*(c.pexecutor), ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType, subClusterType: c.subClusterType})
	if err != nil {
		return err
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
		Values: []string{"nat"},
	})

	tags := []types.Tag{
		{
			Key:   aws.String("Cluster"),
			Value: aws.String(clusterType),
		},
		{
			Key:   aws.String("Type"),
			Value: aws.String("nat"), // tidb/oracle/workstation
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String(clusterName),
		},
	}

	zones, err := getAvailableZones(*c.pexecutor, ctx)
	if err != nil {
		return nil
	}

	cidrBlock := getNextCidr(vpc.CidrBlock, 20)

	// * Subnet preparation
	subnetId, err := SearchSubnet(client, filters)
	if err != nil {
		return err
	}

	if subnetId == nil {
		createSubnetInput := &ec2.CreateSubnetInput{
			VpcId:            &(vpc.VpcId),
			AvailabilityZone: &(zones.Zones[0].ZoneName),
			CidrBlock:        &cidrBlock,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: "subnet",
					Tags:         tags,
				},
			},
		}

		subnetResult, err := client.CreateSubnet(context.TODO(), createSubnetInput)
		if err != nil {
			return err
		}

		subnetId = subnetResult.Subnet.SubnetId
	}

	routeTableId, err := SearchRouteTable(client, filters)
	if err != nil {
		return err
	}

	if routeTableId == nil {
		createRouteTableInput := &ec2.CreateRouteTableInput{
			VpcId: &(vpc.VpcId),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: "route-table",
					Tags:         tags,
				},
			},
		}

		routeTableResult, err := client.CreateRouteTable(context.TODO(), createRouteTableInput)

		if err != nil {
			return err
		}
		routeTableId = routeTableResult.RouteTable.RouteTableId
	}

	internetGatewayId, internetGatewayState, _, err := SearchInternetGateway(client, filters)

	if internetGatewayId == nil {
		createInternetGatewayInput := &ec2.CreateInternetGatewayInput{
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: "internet-gateway",
					Tags:         tags,
				},
			},
		}
		internetGatewayResult, err := client.CreateInternetGateway(context.TODO(), createInternetGatewayInput)
		if err != nil {
			return err
		}

		internetGatewayId = internetGatewayResult.InternetGateway.InternetGatewayId
	}

	associateRouteTableInput := &ec2.AssociateRouteTableInput{
		RouteTableId: routeTableId,
		SubnetId:     subnetId,
	}

	associateRouteTableResult, err := client.AssociateRouteTable(context.TODO(), associateRouteTableInput)
	if err != nil {
		log.Errorf("Failed to associate route table <%#v>  ", associateRouteTableResult)
		return err
	}

	if internetGatewayId != nil && internetGatewayState == nil {
		attachInternetGatewayInput := &ec2.AttachInternetGatewayInput{
			InternetGatewayId: internetGatewayId,
			VpcId:             &(vpc.VpcId),
		}

		_, err := client.AttachInternetGateway(context.TODO(), attachInternetGatewayInput)
		if err != nil {
			log.Errorf("Failed to attach internet gateway  <%#v>  ", err)
			return err
		}
	}

	elasticAddress, err := SearchAddresses(client, filters)
	if err != nil {
		return err
	}

	if elasticAddress == nil {
		allocateAddressInput := &ec2.AllocateAddressInput{
			NetworkBorderGroup: aws.String(cfg.Region),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: "elastic-ip",
					Tags:         tags,
				},
			},
		}
		allocateAddressResult, err := client.AllocateAddress(context.TODO(), allocateAddressInput)
		if err != nil {
			return err
		}
		elasticAddress = allocateAddressResult.AllocationId
	}

	natGatewayId, err := SearchNatGateway(client, filters)
	if err != nil {
		return err
	}

	if natGatewayId == nil {
		createNatGatewayInput := &ec2.CreateNatGatewayInput{
			SubnetId:     subnetId,
			AllocationId: elasticAddress,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: "natgateway",
					Tags:         tags,
				},
			},
		}
		natGatewayResult, err := client.CreateNatGateway(context.TODO(), createNatGatewayInput)
		if err != nil {
			return err
		}
		natGatewayId = natGatewayResult.NatGateway.NatGatewayId
		for cnt := 0; cnt < 10; cnt++ {
			time.Sleep(15 * time.Second)
			natGatewayId, err := SearchNatGateway(client, filters)
			if err != nil {
				return err
			}
			if natGatewayId != nil {
				break
			}
		}
	}

	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         routeTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            internetGatewayId,
	}

	_, err = client.CreateRoute(context.TODO(), createRouteInput)
	if err != nil {
		//		fmt.Printf("The route creation error is ", createRouteResult)
		return err
	}

	return nil
}

type DestroyNAT struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

func (c *DestroyNAT) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

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
		Values: []string{"nat"},
	})

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	// Destroy nat gateway
	natGatewayId, err := SearchNatGateway(client, filters)
	if err != nil {
		return err
	}

	if natGatewayId != nil {
		deleteNatGatewayInput := &ec2.DeleteNatGatewayInput{NatGatewayId: natGatewayId}

		_, err = client.DeleteNatGateway(context.TODO(), deleteNatGatewayInput)
		if err != nil {
			return err
		}

		for cnt := 0; cnt < 10; cnt++ {
			time.Sleep(15 * time.Second)
			natGatewayId, err := SearchNatGateway(client, filters)
			if err != nil {
				return err
			}
			if natGatewayId == nil {
				break
			}
		}
	}

	// Destroy subnet
	subnetId, err := SearchSubnet(client, filters)
	if err != nil {
		return err
	}

	if subnetId != nil {
		deleteSubnetInput := &ec2.DeleteSubnetInput{SubnetId: subnetId}

		if _, err := client.DeleteSubnet(context.TODO(), deleteSubnetInput); err != nil {
			return err
		}
	}

	// Destroy route table
	routeTableId, err := SearchRouteTable(client, filters)
	if err != nil {
		return err
	}

	if routeTableId != nil {
		deleteRouteTableInput := &ec2.DeleteRouteTableInput{RouteTableId: routeTableId}

		if _, err := client.DeleteRouteTable(context.TODO(), deleteRouteTableInput); err != nil {
			return err
		}
	}

	// Destroy internet gateway
	internetGatewayId, internetGatewayState, vpcId, err := SearchInternetGateway(client, filters)
	if err != nil {
		return err
	}

	if internetGatewayState != nil {
		detachInternetGatewayInput := &ec2.DetachInternetGatewayInput{InternetGatewayId: internetGatewayId, VpcId: vpcId}

		if _, err := client.DetachInternetGateway(context.TODO(), detachInternetGatewayInput); err != nil {
			return err
		}

	}

	if internetGatewayId != nil {
		deleteInternetGatewayInput := &ec2.DeleteInternetGatewayInput{InternetGatewayId: internetGatewayId}

		if _, err := client.DeleteInternetGateway(context.TODO(), deleteInternetGatewayInput); err != nil {
			return err
		}

	}

	// Destroy ip address
	allocationId, err := SearchAddresses(client, filters)
	if err != nil {
		return err
	}

	if subnetId != nil {
		releaseAddressInput := &ec2.ReleaseAddressInput{AllocationId: allocationId}

		if _, err := client.ReleaseAddress(context.TODO(), releaseAddressInput); err != nil {
			return err
		}

	}

	return nil
}

func SearchSubnet(client *ec2.Client, filters []types.Filter) (*string, error) {
	input := &ec2.DescribeSubnetsInput{Filters: filters}

	result, err := client.DescribeSubnets(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	if len((*result).Subnets) > 1 {
		return nil, errors.New("More than required subnets")
	}

	if len((*result).Subnets) == 0 {
		return nil, nil
	}

	return (*result).Subnets[0].SubnetId, nil
}

func SearchRouteTable(client *ec2.Client, filters []types.Filter) (*string, error) {
	input := &ec2.DescribeRouteTablesInput{
		Filters: filters,
	}

	result, err := client.DescribeRouteTables(context.TODO(), input)
	if err != nil {
		//		fmt.Printf("The error is <%#v> \n\n\n", err)
		return nil, err
	}

	//	fmt.Printf("The result from the route table creation <%#v> \n\n\n", result.RouteTables)
	if len((*result).RouteTables) > 1 {
		return nil, errors.New("More than required route table")
	}

	if len((*result).RouteTables) == 0 {
		return nil, nil
	}

	return (*result).RouteTables[0].RouteTableId, nil
}

func SearchInternetGateway(client *ec2.Client, filters []types.Filter) (*string, *types.AttachmentStatus, *string, error) {
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: filters,
	}

	result, err := client.DescribeInternetGateways(context.TODO(), input)
	if err != nil {
		//		fmt.Printf("The error is <%#v> \n\n\n", err)
		return nil, nil, nil, err
	}

	//	fmt.Printf("The result from the internet gateway creation <%#v> \n\n\n", result)
	if len((*result).InternetGateways) > 1 {
		return nil, nil, nil, errors.New("More than required route table")
	}

	if len((*result).InternetGateways) == 0 {
		return nil, nil, nil, nil
	}

	//	fmt.Printf("The attached vpc info is <%#v> \n\n\n", (*result).InternetGateways[0].Attachments)

	if len((*result).InternetGateways[0].Attachments) > 0 {
		return (*result).InternetGateways[0].InternetGatewayId, &(((*result).InternetGateways[0].Attachments[0]).State), (*result).InternetGateways[0].Attachments[0].VpcId, nil
	} else {
		return (*result).InternetGateways[0].InternetGatewayId, nil, nil, nil
	}

	// fmt.Printf("The number of the subnets is <%d> \n\n\n", len((*result).Subnets))
	// fmt.Printf("The error is <%#v> \n\n\n", result)

}

func SearchAddresses(client *ec2.Client, filters []types.Filter) (*string, error) {
	input := &ec2.DescribeAddressesInput{
		Filters: filters,
	}

	result, err := client.DescribeAddresses(context.TODO(), input)
	if err != nil {
		//		fmt.Printf("The error is <%#v> \n\n\n", err)
		return nil, err
	}

	//	fmt.Printf("The result from the internet gateway creation <%#v> \n\n\n", result)
	if len((*result).Addresses) > 1 {
		return nil, errors.New("More than required route table")
	}

	if len((*result).Addresses) == 0 {
		return nil, nil
	}

	//	fmt.Printf("The addresses is <%#v> \n\n\n", result)
	return (*result).Addresses[0].AllocationId, nil
}

func SearchNatGateway(client *ec2.Client, filters []types.Filter) (*string, error) {
	input := &ec2.DescribeNatGatewaysInput{
		Filter: filters,
	}

	result, err := client.DescribeNatGateways(context.TODO(), input)
	if err != nil {
		//		fmt.Printf("The error is <%#v> \n\n\n", err)
		return nil, err
	}

	//	fmt.Printf("The result from the internet gateway creation <%#v> \n\n\n", result)
	if len((*result).NatGateways) > 1 {
		return nil, errors.New("More than required route table")
	}

	if len((*result).NatGateways) == 0 {
		return nil, nil
	}

	//	fmt.Printf("The addresses is <%#v> \n\n\n", result)
	return (*result).NatGateways[0].NatGatewayId, nil
}

// Rollback implements the Task interface
func (c *CreateNAT) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNAT) String() string {
	return fmt.Sprintf("Echo: Creating network ")
}

// Rollback implements the Task interface
func (c *DestroyNAT) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyNAT) String() string {
	return fmt.Sprintf("Echo: Destroying NAT ")
}
