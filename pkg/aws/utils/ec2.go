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

package utils

import (
	"context"
	// "encoding/json"
	// "errors"
	"fmt"
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
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "go.uber.org/zap"
)

func ExtractEC2Instances(clusterName, clusterType, subClusterType string) (*map[string][]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{clusterType}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{clusterName}})
	if subClusterType != "" {
		filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{subClusterType}})
	}

	describeInstances, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}

	mapInstances := make(map[string][]string)

	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				fmt.Printf("The tag key: <%s> vs <%s> \n\n\n\n\n\n", *tag.Key, *tag.Value)
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
				// if *tag.Key == "Component" && *tag.Value == "dm-master" {
				// 	mapInstances["DMMaster"] = append(mapInstances["DMMaster"], *instance.PrivateIpAddress)
				// }
				// if *tag.Key == "Component" && *tag.Value == "dm-worker" {
				// 	mapInstances["DMMaster"] = append(mapInstances["DMMaster"], *instance.PrivateIpAddress)
				// }

				// if tag["Key"] == "Component" && tag["Value"] == "dm-master" {
				// 	fmt.Printf("The ip is: %s \n", instance.PrivateIpAddress)
				// 	// tplDMData.DMMaster = append(tplDMData.DMMaster, instance.PrivateIpAddress)
				// }

				// if tag["Key"] == "Component" && tag["Value"] == "dm-worker" {
				// 	// tplDMData.DMWorker = append(tplDMData.DMWorker, instance.PrivateIpAddress)
				// }

				// if tag["Key"] == "Component" && tag["Value"] == "workstation" {
				// 	// tplDMData.Grafana = append(tplDMData.Grafana, instance.PrivateIpAddress)
				// }

				// if tag["Key"] == "Component" && tag["Value"] == "workstation" {
				// 	// tplDMData.Monitor = append(tplDMData.Monitor, instance.PrivateIpAddress)

				// }
				// if tag["Key"] == "Component" && tag["Value"] == "workstation" {
				// 	// tplDMData.AlertManager = append(tplDMData.AlertManager, instance.PrivateIpAddress)

				// }
			}

			//			fmt.Printf("Instance: ", instance)
		}
	}
	// if len(describeInstances.Reservations) > 0 {
	// 	if checkStatus == true {
	// 		for _, reservation := range describeInstances.Reservations {
	// 			for _, instance := range reservation.Instances {

	// 				// Check the status
	// 				describeInstanceStatus, err := client.DescribeInstanceStatus(context.TODO(), &ec2.DescribeInstanceStatusInput{
	// 					InstanceIds: []string{*instance.InstanceId},
	// 				})
	// 				if err != nil {
	// 					return false, err
	// 				}

	// 				if len(describeInstanceStatus.InstanceStatuses) > 0 {
	// 					for _, instanceStatus := range describeInstanceStatus.InstanceStatuses {
	// 						if instanceStatus.InstanceStatus.Status == types.SummaryStatusOk {
	// 							return true, nil
	// 						}

	// 					}
	// 				}
	// 				return false, nil
	// 			}
	// 		}
	// 	} else {
	// 		return true, nil
	// 	}
	// }

	return &mapInstances, nil
}
