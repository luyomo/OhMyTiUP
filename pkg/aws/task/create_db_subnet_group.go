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
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"strings"
)

type DBSubnetGroup struct {
	DBSubnetGroupName string `json:"DBSubnetGroupName"`
	VpcId             string `json:"VpcId"`
	SubnetGroupStatus string `json:"SubnetGroupStatus"`
	DBSubnetGroupArn  string `json:"DBSubnetGroupArn"`
}

type DBSubnetGroups struct {
	DBSubnetGroups []DBSubnetGroup `json:"DBSubnetGroups"`
}

type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type TagList struct {
	TagList []Tag `json:"TagList"`
}

type CreateDBSubnetGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBSubnetGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DB Subnet group '%s' not found", c.clusterName)) {
			fmt.Printf("The DB Cluster has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		fmt.Printf("The data is <%s> \n\n\n", string(command))

		var dbSubnetGroups DBSubnetGroups
		if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}

		for _, subnetGroups := range dbSubnetGroups.DBSubnetGroups {
			existsResource := ExistsResource(c.clusterType, c.subClusterType, c.clusterName, subnetGroups.DBSubnetGroupArn, local, ctx)
			if existsResource == true {
				return nil
			}
		}
	}

	var subnets []string
	for _, subnet := range c.clusterInfo.privateSubnets {
		subnets = append(subnets, "\""+subnet+"\"")
	}
	command = fmt.Sprintf("aws rds create-db-subnet-group --db-subnet-group-name %s --db-subnet-group-description \"%s\" --subnet-ids '\"'\"'[%s]'\"'\"' --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", c.clusterName, c.clusterName, strings.Join(subnets, ","), c.clusterName, c.clusterType, c.subClusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The db subnets group is <%s>\n\n\n", stdout)

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBSubnetGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func groupExists(groupName string, subnetGroups []DBSubnetGroup) bool {
	for _, theSubnetGroup := range subnetGroups {
		if groupName == theSubnetGroup.DBSubnetGroupName {
			return true
		}
	}
	return false
}

func ExistsResource(clusterType, subClusterType, clusterName, resourceName string, executor ctxt.Executor, ctx context.Context) bool {
	command := fmt.Sprintf("aws rds list-tags-for-resource --resource-name %s ", resourceName)
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
