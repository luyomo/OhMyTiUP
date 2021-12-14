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
	"github.com/luyomo/tisample/pkg/executor"
	"strings"
)

type DestroyDBClusterParameterGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBClusterParameterGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-cluster-parameter-groups --db-cluster-parameter-group-name '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBClusterParameterGroup not found") {
			// If there is no resource, go ahead
			fmt.Printf("The DB Cluster Parameter group has not created.\n\n\n")
		} else {
			fmt.Printf("ERRORS: describe-db-cluster-parameter-groups <%s>", string(stderr))
			return err
		}
	} else {
		var dbClusterParameterGroups DBClusterParameterGroups
		if err = json.Unmarshal(stdout, &dbClusterParameterGroups); err != nil {
			fmt.Printf("ERRORS: describe-db-cluster-parameter-groups json parsing <%s>", string(stderr))
			return err
		}
		fmt.Printf("The db cluster parameter groups is <%#v> \n\n\n", dbClusterParameterGroups)
		for _, dbClusterParameterGroup := range dbClusterParameterGroups.DBClusterParameterGroups {
			existsResource := ExistsResource(c.clusterType, c.subClusterType, c.clusterName, dbClusterParameterGroup.DBClusterParameterGroupArn, local, ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-cluster-parameter-group --db-cluster-parameter-group-name %s", c.clusterName)

				stdout, stderr, err = local.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("The comamnd is <%s> \n\n\n", command)
					fmt.Printf("ERRORS: delete-db-cluster-parameter-group  <%s> \n\n\n", string(stderr))
					return err
				}
				return nil
			}
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBClusterParameterGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBClusterParameterGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
