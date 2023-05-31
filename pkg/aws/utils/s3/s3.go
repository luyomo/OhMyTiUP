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

package s3

import (
	"context"
	// "encoding/json"
	// "errors"
	// "fmt"
	// "os"
	// "path"
	// "sort"
	// "strings"
	// "text/template"
	// "time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
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

type S3API struct {
	client *s3.Client

	mapArgs *map[string]string
}

func NewS3API(mapArgs *map[string]string) (*S3API, error) {
	s3api := S3API{}

	if mapArgs != nil {
		s3api.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	s3api.client = s3.NewFromConfig(cfg)

	return &s3api, nil
}

func (c *S3API) GetObject(bucket, key string) error {
	_, err := c.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	return nil
}

// func (e *EC2API) GetAvailabilitySubnet4EndpointService(serviceName string) (*[]string, error) {
// 	availableZones, err := e.getEndpointServiceAvailabilityZones(serviceName)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// fmt.Printf("The available zones are: %#v \n\n\n", availableZones)

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

// func (e *EC2API) ExtractEC2Instances(clusterName, clusterType, subClusterType string) (*map[string][]string, error) {

// 	// var filters []types.Filter
// 	// filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{clusterType}})
// 	// filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{clusterName}})
// 	// if subClusterType != "" {
// 	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{subClusterType}})
// 	// }
// 	filters := e.makeFilters()

// 	describeInstances, err := e.client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: *filters})
// 	if err != nil {
// 		return nil, err
// 	}

// 	mapInstances := make(map[string][]string)

// 	for _, reservation := range describeInstances.Reservations {
// 		for _, instance := range reservation.Instances {
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
// 				}
// 			}
// 		}
// 	}

// 	return &mapInstances, nil
// }

func (c *S3API) makeTags() *[]types.Tag {
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

// func (c *S3API) makeFilters() *[]types.Filter {
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
