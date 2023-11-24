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
	"encoding/json"
	"fmt"

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	//	"github.com/luyomo/OhMyTiUP/pkg/executor"
	//	"strings"
)

type Endpoint struct {
	EndpointIdentifier string `json:"EndpointIdentifier"`
	EndpointType       string `json:"EndpointType"`
	EngineName         string `json:"EngineName"`
	Status             string `json:"Status"`
	EndpointArn        string `json:"EndpointArn"`
}
type Endpoints struct {
	Endpoints []Endpoint `json:"Endpoints"`
}

type EndpointRecord struct {
	Endpoint Endpoint `json:"Endpoint"`
}

type ReplicationInstance struct {
	ReplicationInstanceIdentifier       string `json:"ReplicationInstanceIdentifier"`
	ReplicationInstanceClass            string `json:"ReplicationInstanceClass"`
	ReplicationInstanceStatus           string `json:"ReplicationInstanceStatus"`
	ReplicationInstanceArn              string `json:"ReplicationInstanceArn"`
	ReplicationInstancePrivateIpAddress string `json:"ReplicationInstancePrivateIpAddress"`
}

type ReplicationInstances struct {
	ReplicationInstances []ReplicationInstance `json:"ReplicationInstances"`
}

type ReplicationInstanceRecord struct {
	ReplicationInstance ReplicationInstance `json:"ReplicationInstance"`
}

type DMSSubnetGroup struct {
	ReplicationSubnetGroupIdentifier string `json:"ReplicationSubnetGroupIdentifier"`
	VpcId                            string `json:"VpcId"`
	SubnetGroupStatus                string `json:"SubnetGroupStatus"`
}

type DMSSubnetGroups struct {
	DMSSubnetGroups []DMSSubnetGroup `json:"ReplicationSubnetGroups"`
}

type ReplicationTask struct {
	ReplicationTaskIdentifier string `json:"ReplicationTaskIdentifier"`
	SourceEndpointArn         string `json:"SourceEndpointArn"`
	TargetEndpointArn         string `json:"TargetEndpointArn"`
	ReplicationInstanceArn    string `json:"ReplicationInstanceArn"`
	MigrationType             string `json:"MigrationType"`
	Status                    string `json:"Status"`
	ReplicationTaskArn        string `json:"ReplicationTaskArn"`
}

type ReplicationTaskRecord struct {
	ReplicationTask ReplicationTask `json:"ReplicationTask"`
}

type ReplicationTasks struct {
	ReplicationTasks []ReplicationTask `json:"ReplicationTasks"`
}

var DMSInfo struct {
	SourceEndpointArn      string
	TargetEndpointArn      string
	ReplicationInstanceArn string
}

func ExistsDMSResource(clusterType, subClusterType, clusterName, resourceName string, executor ctxt.Executor, ctx context.Context) bool {
	command := fmt.Sprintf("aws dms list-tags-for-resource --resource-arn %s ", resourceName)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return false
	}

	var tagList TagList
	if err = json.Unmarshal(stdout, &tagList); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return false
	}
	matchedCnt := 0
	for _, tag := range tagList.TagList {
		if tag.Key == "Cluster" && tag.Value == clusterType {
			matchedCnt++
		}
		if tag.Key == "Type" && tag.Value == subClusterType {
			matchedCnt++
		}
		if tag.Key == "Name" && tag.Value == clusterName {
			matchedCnt++
		}
		if matchedCnt == 3 {
			return true
		}
	}
	return false
}
