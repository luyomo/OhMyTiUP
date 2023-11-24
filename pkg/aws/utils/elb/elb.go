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

package elb

import (
	"context"
	"errors"
	"fmt"

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	// "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	smithy "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
)

// func MapTag() *map[string]string {
// 	return &map[string]string{
// 		"clusterName":    "Name",
// 		"clusterType":    "Cluster",
// 		"subClusterType": "Type",
// 		"scope":          "Scope",
// 		"component":      "Component",
// 	}
// }

type ELBAPI struct {
	client *elb.Client

	mapArgs *map[string]string
}

func NewELBAPI(mapArgs *map[string]string) (*ELBAPI, error) {
	elbapi := ELBAPI{}

	if mapArgs != nil {
		elbapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := elb.NewFromConfig(cfg)
	elbapi.client = client

	return &elbapi, nil
}

func (e *ELBAPI) GetNLB(clusterName string) (*string, error) {
	describeLoadBalancers, err := e.client.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{Names: []string{clusterName}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "LoadBalancerNotFound" {
				return nil, nil
			}
		}

		return nil, err
	}
	if len(describeLoadBalancers.LoadBalancers) > 1 {
		return nil, errors.New("Multiple ELB found. ")
	}

	return describeLoadBalancers.LoadBalancers[0].DNSName, nil
}

// func (e *EC2API) GetAvailabilitySubnet4EndpointService(serviceName string) (*[]string, error) {
// 	availableZones, err := e.getEndpointServiceAvailabilityZones(serviceName)
// 	if err != nil {
// 		return nil, err
// 	}

// 	filters := e.makeFilters()

// 	resp, err := e.client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{Filters: *filters})
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, subnet := range resp.Subnets {
// 		if utils.Includes(*availableZones, *subnet.AvailabilityZone) == true {
// 			return &[]string{*subnet.SubnetId}, nil
// 		}

// 	}

// 	return nil, errors.New("Not availability zone for service found")
// }
// func (e *EC2API) getEndpointServiceAvailabilityZones(serviceName string) (*[]string, error) {
// 	resp, err := e.client.DescribeVpcEndpointServices(context.TODO(), &ec2.DescribeVpcEndpointServicesInput{
// 		ServiceNames: []string{serviceName},
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(resp.ServiceDetails) == 0 {
// 		return nil, errors.New("No endpoint service found")
// 	}

// 	if len(resp.ServiceDetails) > 1 {
// 		return nil, errors.New("More than one endpoint service found")
// 	}

// 	return &resp.ServiceDetails[0].AvailabilityZones, nil
// }

// func (e *EC2API) ExtractEC2Instances() (*map[string][]string, error) {

// 	filters := e.makeFilters()

// 	describeInstances, err := e.client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: *filters})
// 	if err != nil {
// 		return nil, err
// 	}

// 	mapInstances := make(map[string][]string)

// 	for _, reservation := range describeInstances.Reservations {
// 		for _, instance := range reservation.Instances {
// 			if instance.State.Name == types.InstanceStateNameTerminated {
// 				continue
// 			}

// 			for _, tag := range instance.Tags {
// 				switch {
// 				case *tag.Key == "Component" && *tag.Value == "dm-master":
// 					mapInstances["DMMaster"] = append(mapInstances["DMMaster"], *instance.PrivateIpAddress)
// 				case *tag.Key == "Component" && *tag.Value == "dm-worker":
// 					mapInstances["DMWorker"] = append(mapInstances["DMWorker"], *instance.PrivateIpAddress)
// 				case *tag.Key == "Component" && *tag.Value == "workstation":
// 					mapInstances["Grafana"] = append(mapInstances["Grafana"], *instance.PrivateIpAddress)
// 					mapInstances["Monitor"] = append(mapInstances["Monitor"], *instance.PrivateIpAddress)
// 					mapInstances["AlertManager"] = append(mapInstances["AlertManager"], *instance.PrivateIpAddress)
// 				case *tag.Key == "Component" && *tag.Value == "mysql-worker":
// 					mapInstances["MySQLWorker"] = append(mapInstances["MySQLWorker"], *instance.PrivateIpAddress)
// 				}
// 			}
// 		}
// 	}

// 	return &mapInstances, nil
// }

// func (c *EC2API) makeTags() *[]types.Tag {
// 	var tags []types.Tag
// 	if c.mapArgs == nil {
// 		return &tags
// 	}

// 	for key, tagName := range *(MapTag()) {
// 		if tagValue, ok := (*c.mapArgs)[key]; ok {
// 			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
// 		}
// 	}

// 	return &tags
// }

// func (c *EC2API) makeFilters() *[]types.Filter {
// 	var filters []types.Filter
// 	if c.mapArgs == nil {
// 		return &filters
// 	}

// 	for key, tagName := range *(MapTag()) {
// 		if tagValue, ok := (*c.mapArgs)[key]; ok {
// 			filters = append(filters, types.Filter{Name: aws.String("tag:" + tagName), Values: []string{tagValue}})
// 		}
// 	}

// 	return &filters
// }
