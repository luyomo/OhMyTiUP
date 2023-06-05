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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type TiDBClusterNodes struct {
	PD      []string
	TiDB    []string
	TiKV    []string
	TiCDC   []string
	DM      []string
	Monitor []string
	Pump    []string
	Drainer []string
}

func ExtractTiDBClusterNodes(name, cluster, clusterType string) (*TiDBClusterNodes, error) {

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{name}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{cluster}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{clusterType}})
	filters = append(filters, types.Filter{Name: aws.String("instance-state-name"), Values: []string{"running"}})

	ec2DescribeInstancesInput := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	ec2Instances, err := client.DescribeInstances(context.TODO(), ec2DescribeInstancesInput)
	if err != nil {
		return nil, err
	}

	var retValue TiDBClusterNodes

	for _, reservation := range ec2Instances.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if *(tag.Key) == "Component" && *(tag.Value) == "pd" {
					retValue.PD = append(retValue.PD, *(instance.PrivateIpAddress))
				}
				if *(tag.Key) == "Component" && *(tag.Value) == "tidb" {
					retValue.TiDB = append(retValue.TiDB, *(instance.PrivateIpAddress))
				}
				if *(tag.Key) == "Component" && *(tag.Value) == "tikv" {
					retValue.TiKV = append(retValue.TiKV, *(instance.PrivateIpAddress))
				}
				if *(tag.Key) == "Component" && *(tag.Value) == "ticdc" {
					retValue.TiCDC = append(retValue.TiCDC, *(instance.PrivateIpAddress))
				}

				if *(tag.Key) == "Component" && *(tag.Value) == "dm" {
					retValue.DM = append(retValue.DM, *(instance.PrivateIpAddress))
				}

				if *(tag.Key) == "Component" && *(tag.Value) == "pump" {
					retValue.Pump = append(retValue.Pump, *(instance.PrivateIpAddress))
				}

				if *(tag.Key) == "Component" && *(tag.Value) == "drainer" {
					retValue.Drainer = append(retValue.Drainer, *(instance.PrivateIpAddress))
				}

				if *(tag.Key) == "Component" && *(tag.Value) == "workstation" {
					retValue.Monitor = append(retValue.Monitor, *(instance.PrivateIpAddress))
				}
			}
		}
	}

	return &retValue, nil
}
