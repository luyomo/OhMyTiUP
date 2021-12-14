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

type DestroyDBParameterGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBParameterGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-parameter-groups --db-parameter-group-name '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBParameterGroup not found") {
			fmt.Printf("The DB Parameter group has not created.\n\n\n")
		} else {
			fmt.Printf("ERRORS describe-db-parameter-groups <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var dbParameterGroups DBParameterGroups
		if err = json.Unmarshal(stdout, &dbParameterGroups); err != nil {
			fmt.Printf("ERRORS describe-db-parameter-groups json parsing <%s> \n\n", string(stderr))
			return nil
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", dbParameterGroups)
		for _, dbParameterGroup := range dbParameterGroups.DBParameterGroups {
			fmt.Printf("The cluster info is <%#v> \n\n\n", dbParameterGroup)
			existsResource := ExistsResource(c.clusterType, c.subClusterType, c.clusterName, dbParameterGroup.DBParameterGroupArn, local, ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-parameter-group --db-parameter-group-name %s", c.clusterName)
				fmt.Printf("The comamnd is <%s> \n\n\n", command)
				stdout, stderr, err = local.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("ERRORS delete-db-parameter-group <%s> \n\n\n", string(stderr))
					return err
				}
				fmt.Printf("The db cluster  has exists \n\n\n")
				return nil
			}
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBParameterGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBParameterGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
