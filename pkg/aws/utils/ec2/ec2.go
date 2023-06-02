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
	// "encoding/json"
	"errors"
	// "fmt"
	// "os"
	// "path"
	// "sort"
	// "strings"
	// "text/template"
	// "time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	// astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
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
	// fmt.Printf("The available zones are: %#v \n\n\n", availableZones)

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

func (e *EC2API) ExtractEC2Instances() (*map[string][]string, error) {

	filters := e.makeFilters()

	describeInstances, err := e.client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: *filters})
	if err != nil {
		return nil, err
	}

	mapInstances := make(map[string][]string)

	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
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
				}
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

	// mapTag := map[string]string{
	// 	"clusterName": "Name",
	// 	"clusterType", "Cluster",
	// 	"subClusterType": "Type",
	// 	"scope":          "Scope",
	// 	"component":      "Component",
	// }

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
		}
	}

	// if value, ok := *(c.mapArgs)["clusterType"]; ok {
	// 	tags = append(tags, types.Tag{Key: aws.String("Cluster"), Value: aws.String(value)})
	// }

	// if *(c.mapArgs)["subClusterType"] != nil {
	// 	tags = append(tags, types.Tag{Key: aws.String("Type"), Value: aws.String(*(c.mapArgs)["subClusterType"])})
	// }

	// if *(c.mapArgs)["scope"] != nil {
	// 	tags = append(tags, types.Tag{Key: aws.String("Scope"), Value: aws.String(*(c.mapArgs)["scope"])})
	// }

	// if *(c.mapArgs)["component"] != nil {
	// 	tags = append(tags, types.Tag{Key: aws.String("Component"), Value: aws.String(*(c.mapArgs)["component"])})
	// }

	return &tags
}

func (c *EC2API) makeFilters() *[]types.Filter {
	var filters []types.Filter
	if c.mapArgs == nil {
		return &filters
	}

	// mapTag := map[string]string{
	// 	"clusterName": "Name",
	// 	"clusterType", "Cluster",
	// 	"subClusterType": "Type",
	// 	"scope":          "Scope",
	// 	"component":      "Component",
	// }

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			filters = append(filters, types.Filter{Name: aws.String("tag:" + tagName), Values: []string{tagValue}})
		}
	}

	// if *(c.mapArgs)["clusterName"] != nil {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{*(c.mapArgs)["clusterName"]}})
	// }

	// if *(c.mapArgs)["clusterType"] != nil {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{*(c.mapArgs)["clusterType"]}})
	// }

	// if *(c.mapArgs)["subClusterType"] != nil {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{*(c.mapArgs)["subClusterType"]}})
	// }

	// if *(c.mapArgs)["scope"] != nil {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Scope"), Values: []string{*(c.mapArgs)["scope"]}})
	// }

	// if *(c.mapArgs)["component"] != nil {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Component"), Values: []string{*(c.mapArgs)["component"]}})
	// }

	return &filters
}
