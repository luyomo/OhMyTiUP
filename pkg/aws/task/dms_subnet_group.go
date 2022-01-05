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

	"strings"
)

type CreateDMSSubnetGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSSubnetGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	command := fmt.Sprintf("aws dms describe-replication-subnet-groups --filters \"Name=replication-subnet-group-id,Values=%s\"", clusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error err here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
		return err
	} else {

		var dmsSubnetGroups DMSSubnetGroups
		if err = json.Unmarshal(stdout, &dmsSubnetGroups); err != nil {
			return err
		}

		for len(dmsSubnetGroups.DMSSubnetGroups) > 0 {
			return nil
		}
	}

	var subnets []string
	for _, subnet := range c.clusterInfo.privateSubnets {
		subnets = append(subnets, "\""+subnet+"\"")
	}
	command = fmt.Sprintf("aws dms create-replication-subnet-group --replication-subnet-group-identifier %s --replication-subnet-group-description \"%s\" --subnet-ids '\"'\"'[%s]'\"'\"' --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, clusterName, strings.Join(subnets, ","), clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSSubnetGroup) String() string {
	return fmt.Sprintf("Echo: Creating DMS Subnet Group ")
}

/******************************************************************************/

type DestroyDMSSubnetGroup struct {
	user           string
	host           string
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDMSSubnetGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	// Get the available zones
	command := fmt.Sprintf("aws dms describe-replication-subnet-groups --filters \"Name=replication-subnet-group-id,Values=%s\"", clusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("ERRORS describe-replication-subnet-groups <%s> \n\n\n", string(stderr))
		return err
	} else {
		var dmsSubnetGroups DMSSubnetGroups
		if err = json.Unmarshal(stdout, &dmsSubnetGroups); err != nil {
			fmt.Printf("ERRORS describe-replication-subnet-groups json parsing <%s> \n\n\n", string(stderr))
			return err
		}

		for _, subnet := range dmsSubnetGroups.DMSSubnetGroups {
			command = fmt.Sprintf("aws dms delete-replication-subnet-group --replication-subnet-group-identifier %s", subnet.ReplicationSubnetGroupIdentifier)
			stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)

			if err != nil {
				fmt.Printf("ERRORS delete-replication-subnet-group json parsing <%s> \n\n\n", string(stderr))
				return err
			}
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDMSSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSSubnetGroup) String() string {
	return fmt.Sprintf("Echo: Destroying dms subnet group ")
}
