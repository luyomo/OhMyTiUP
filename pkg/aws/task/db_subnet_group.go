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
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBSubnetGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbSubnetGroupName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", dbSubnetGroupName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), fmt.Sprintf("DB Subnet group '%s' not found", dbSubnetGroupName)) {
			return err
		}
	} else {

		var dbSubnetGroups DBSubnetGroups
		if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
			return err
		}

		for _, subnetGroups := range dbSubnetGroups.DBSubnetGroups {
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, subnetGroups.DBSubnetGroupArn, *c.pexecutor, ctx)
			if existsResource == true {
				return nil
			}
		}
	}

	var subnets []string
	for _, subnet := range c.clusterInfo.privateSubnets {
		subnets = append(subnets, "\""+subnet+"\"")
	}
	command = fmt.Sprintf("aws rds create-db-subnet-group --db-subnet-group-name %s --db-subnet-group-description \"%s\" --subnet-ids '\"'\"'[%s]'\"'\"' --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", dbSubnetGroupName, clusterName, strings.Join(subnets, ","), clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBSubnetGroup) String() string {
	return fmt.Sprintf("Echo: Creating DB Subnet Group ")
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
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return false
	}

	var tagList TagList
	if err = json.Unmarshal(stdout, &tagList); err != nil {
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

/******************************************************************************/

type DestroyDBSubnetGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBSubnetGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbSubnetGroupName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", dbSubnetGroupName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DB Subnet group '%s' not found", dbSubnetGroupName)) {
			return nil
		} else {
			return err
		}
	}

	var dbSubnetGroups DBSubnetGroups
	if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
		return err
	}

	for _, subnetGroups := range dbSubnetGroups.DBSubnetGroups {
		existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, subnetGroups.DBSubnetGroupArn, *(c.pexecutor), ctx)
		if existsResource == true {
			command = fmt.Sprintf("aws rds delete-db-subnet-group --db-subnet-group-name %s", dbSubnetGroupName)

			stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBSubnetGroup) String() string {
	return fmt.Sprintf("Echo: Destroyng db subnet group")
}
