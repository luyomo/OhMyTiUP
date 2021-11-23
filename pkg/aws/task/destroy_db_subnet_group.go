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
	//	"strings"
)

type DestroyDBSubnetGroup struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroyDBSubnetGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The data is <%s> \n\n\n", string(command))

	var dbSubnetGroups DBSubnetGroups
	if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	fmt.Printf("The db subnet groups is <%#v> \n\n\n", dbSubnetGroups)
	for _, subnetGroups := range dbSubnetGroups.DBSubnetGroups {
		existsResource := ExistsResource(c.clusterType, c.clusterName, subnetGroups.DBSubnetGroupArn, local, ctx)
		if existsResource == true {
			command = fmt.Sprintf("aws rds delete-db-subnet-group --db-subnet-group-name %s", c.clusterName)
			fmt.Printf("The comamnd is <%s> \n\n\n", command)
			stdout, stderr, err = local.Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return nil
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
