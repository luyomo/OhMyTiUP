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
	// "fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/embed"
	// operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	// "go.uber.org/zap"
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
