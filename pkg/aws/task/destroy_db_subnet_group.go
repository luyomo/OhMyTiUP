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
	//	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"strings"
)

type DestroyDBSubnetGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBSubnetGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DB Subnet group '%s' not found", c.clusterName)) {
			fmt.Printf("The DB Cluster has not created.\n\n\n")
			return nil
		} else {
			fmt.Printf("ERRORS: describe-db-subnet-groups <%s> \n\n\n", string(stderr))
			return err
		}
	}

	var dbSubnetGroups DBSubnetGroups
	if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
		fmt.Printf("ERRORS: describe-db-subnet-groups json parsing  <%s> \n\n", string(stderr))
		return err
	}

	fmt.Printf("The db subnet groups is <%#v> \n\n\n", dbSubnetGroups)
	for _, subnetGroups := range dbSubnetGroups.DBSubnetGroups {
		existsResource := ExistsResource(c.clusterType, c.subClusterType, c.clusterName, subnetGroups.DBSubnetGroupArn, local, ctx)
		if existsResource == true {
			command = fmt.Sprintf("aws rds delete-db-subnet-group --db-subnet-group-name %s", c.clusterName)

			stdout, stderr, err = local.Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The comamnd is <%s> \n\n\n", command)
				fmt.Printf("ERRORS: delete-db-subnet-group <%s> \n\n", string(stderr))
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
