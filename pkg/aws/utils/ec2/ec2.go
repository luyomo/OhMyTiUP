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

package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
)

func MapTag() *map[string]string {
	return &map[string]string{
		"clusterName":    "Name",
		"clusterType":    "Cluster",
		"subClusterType": "Type",
		"scope":          "Scope",
		"component":      "Component",
	}
}

type EC2API struct {
	client *ec2.Client

	mapArgs *map[string]string
}

func NewEC2API(mapArgs *map[string]string) (*EC2API, error) {
	ec2api := EC2API{}

	if mapArgs != nil {
		ec2api.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)
	ec2api.client = client

	return &ec2api, nil
}

func (e *EC2API) GetAvailabilitySubnet4EndpointService(serviceName string) (*[]string, error) {
	availableZones, err := e.getEndpointServiceAvailabilityZones(serviceName)
	if err != nil {
		return nil, err
	}

	filters := e.makeFilters()

	resp, err := e.client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	for _, subnet := range resp.Subnets {
		if utils.Includes(*availableZones, *subnet.AvailabilityZone) == true {
			return &[]string{*subnet.SubnetId}, nil
		}

	}

	return nil, errors.New("Not availability zone for service found")
}
func (e *EC2API) getEndpointServiceAvailabilityZones(serviceName string) (*[]string, error) {
	resp, err := e.client.DescribeVpcEndpointServices(context.TODO(), &ec2.DescribeVpcEndpointServicesInput{
		ServiceNames: []string{serviceName},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.ServiceDetails) == 0 {
		return nil, errors.New("No endpoint service found")
	}

	if len(resp.ServiceDetails) > 1 {
		return nil, errors.New("More than one endpoint service found")
	}

	return &resp.ServiceDetails[0].AvailabilityZones, nil
}

func (e *EC2API) GetVpcId() (*types.Vpc, error) {
	filters := e.makeFilters()

	describeVpc, err := e.client.DescribeVpcs(context.TODO(), &ec2.DescribeVpcsInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	if len(describeVpc.Vpcs) > 1 {
		return nil, errors.New(fmt.Sprintf("Multiple VPCs found: %#v", describeVpc))
	}

	if len(describeVpc.Vpcs) == 0 {
		return nil, nil
	}

	return &(describeVpc.Vpcs[0]), nil

}

func (e *EC2API) GetRouteTable() (*types.RouteTable, error) {
	filters := e.makeFilters()

	describeRouteTables, err := e.client.DescribeRouteTables(context.TODO(), &ec2.DescribeRouteTablesInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	if len(describeRouteTables.RouteTables) > 1 {
		return nil, errors.New(fmt.Sprintf("Multiple VPCs found: %#v", describeRouteTables))
	}

	if len(describeRouteTables.RouteTables) == 0 {
		return nil, nil
	}

	return &describeRouteTables.RouteTables[0], nil
}

func (e *EC2API) GetTransitGateway() (*types.TransitGateway, error) {
	filters := e.makeFilters()

	*filters = append(*filters, types.Filter{Name: aws.String("state"), Values: []string{"available"}})

	describeTransitGateways, err := e.client.DescribeTransitGateways(context.TODO(), &ec2.DescribeTransitGatewaysInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	// fmt.Printf("Transit gateways : %#v \n\n\n", describeTransitGateways)

	if len(describeTransitGateways.TransitGateways) > 1 {
		return nil, errors.New(fmt.Sprintf("Multiple transit gateways found: %#v", describeTransitGateways))
	}

	if len(describeTransitGateways.TransitGateways) == 0 {
		return nil, nil
	}

	return &describeTransitGateways.TransitGateways[0], nil

}

/*
Input: filters

	target vpc's cidr block
	transitgatewayid
*/
func (e *EC2API) CreateRoute(cidr, transitGatewayId string) error {
	routeTable, err := e.GetRouteTable()
	if err != nil {
		return err
	}
	if routeTable == nil {
		return errors.New("No source route table found.")
	}

	routeHasExists, err := e.routeHasExists(routeTable, cidr, transitGatewayId)
	if err != nil {
		return err
	}

	if routeHasExists == true {
		return nil
	}

	_, err = e.client.CreateRoute(context.TODO(), &ec2.CreateRouteInput{RouteTableId: routeTable.RouteTableId,
		DestinationCidrBlock: aws.String(cidr),
		TransitGatewayId:     aws.String(transitGatewayId)})
	if err != nil {
		return err
	}
	return nil

}

func (e *EC2API) routeHasExists(routeTable *types.RouteTable, cidr, transitGatewayId string) (bool, error) {

	for _, route := range routeTable.Routes {
		if route.TransitGatewayId != nil && *route.TransitGatewayId == transitGatewayId && cidr == *route.DestinationCidrBlock {
			if route.State == "blackhole" {

				_, err := e.client.DeleteRoute(context.TODO(), &ec2.DeleteRouteInput{RouteTableId: routeTable.RouteTableId, DestinationCidrBlock: aws.String(cidr)})
				if err != nil {
					return false, err
				}
				return false, nil
			}

			return true, nil

		}
	}

	return false, nil
}

func (e *EC2API) ExtractEC2Instances() (*map[string][]interface{}, error) {

	filters := e.makeFilters()

	describeInstances, err := e.client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	mapInstances := make(map[string][]interface{})

	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
			if instance.State.Name == types.InstanceStateNameTerminated {
				continue
			}

			// Only usful for TiKV nodes
			mapTiKVData := make(map[string]interface{})
			mapTiKVData["Labels"] = make(map[string]string)

			for _, tag := range instance.Tags {
				switch {
				case *tag.Key == "Component" && *tag.Value == "dm-master":
					mapInstances["DMMaster"] = append(mapInstances["DMMaster"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "dm-worker":
					mapInstances["DMWorker"] = append(mapInstances["DMWorker"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "workstation":
					mapInstances["Grafana"] = append(mapInstances["Grafana"], *instance.PrivateIpAddress)
					mapInstances["Monitor"] = append(mapInstances["Monitor"], *instance.PrivateIpAddress)
					mapInstances["AlertManager"] = append(mapInstances["AlertManager"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "mysql-worker":
					mapInstances["MySQLWorker"] = append(mapInstances["MySQLWorker"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "pd":
					mapInstances["PD"] = append(mapInstances["PD"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "tidb":
					mapInstances["TiDB"] = append(mapInstances["TiDB"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "tiflash":
					mapInstances["TiFlash"] = append(mapInstances["TiFlash"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "ticdc":
					mapInstances["TiCDC"] = append(mapInstances["TiCDC"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "dm":
					mapInstances["DM"] = append(mapInstances["DM"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "pump":
					mapInstances["Pump"] = append(mapInstances["Pump"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "drainer":
					mapInstances["Drainer"] = append(mapInstances["Drainer"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "monitor":
					mapInstances["Monitor"] = append(mapInstances["Monitor"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "grafana":
					mapInstances["Grafana"] = append(mapInstances["Grafana"], *instance.PrivateIpAddress)
				case *tag.Key == "Component" && *tag.Value == "alert-manager":
					mapInstances["AlertManager"] = append(mapInstances["AlertManager"], *instance.PrivateIpAddress)
				// Below two are used for tikv and labels
				case *tag.Key == "Component" && *tag.Value == "tikv":
					mapTiKVData["IPAddress"] = *instance.PrivateIpAddress
				case strings.Contains(*tag.Key, "label:"):
					tagKey := strings.Replace(*tag.Key, "label:", "", 1)
					mapTiKVData["Labels"].(map[string]string)[tagKey] = *tag.Value

					// Labels is used to confiure the replication.location-labels under pd config
					existsInArray := false
					for _, element := range mapInstances["Labels"] {
						if element == tagKey {
							existsInArray = true
						}
					}
					if existsInArray == false {
						mapInstances["Labels"] = append(mapInstances["Labels"], tagKey)
					}
				}
			}
			if _, ok := mapTiKVData["IPAddress"]; ok {
				mapInstances["TiKV"] = append(mapInstances["TiKV"], mapTiKVData)
			}

		}
	}

	return &mapInstances, nil
}

func (c *EC2API) makeTags() *[]types.Tag {
	var tags []types.Tag
	if c.mapArgs == nil {
		return &tags
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
		}
	}

	return &tags
}

func (c *EC2API) makeFilters() *[]types.Filter {
	var filters []types.Filter
	if c.mapArgs == nil {
		return &filters
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			filters = append(filters, types.Filter{Name: aws.String("tag:" + tagName), Values: []string{tagValue}})
		}
	}

	return &filters
}
