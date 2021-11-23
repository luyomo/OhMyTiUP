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
	//	"github.com/luyomo/tisample/pkg/aurora/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"strconv"
	"strings"
	"time"
)

type DestroyDBInstance struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroyDBInstance) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", c.clusterName)) {
			fmt.Printf("The DB Instance has not created.\n\n\n")
			return nil
		} else {
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		for _, instance := range dbInstances.DBInstances {
			fmt.Printf("The db instance is <%#v> \n\n\n", instance)
			existsResource := ExistsResource(c.clusterType, c.clusterName, instance.DBInstanceArn, local, ctx)
			if existsResource == true {
				fmt.Printf("The db cluster  has exists \n\n\n")
				fmt.Printf("The cluster name is <%#v> \n\n\n", instance)
				command = fmt.Sprintf("aws rds delete-db-instance --db-instance-identifier %s", c.clusterName)
				fmt.Printf("The comamnd is <%s> \n\n\n", command)
				_, stderr, err = local.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("The error here is <%#v> \n\n", err)
					fmt.Printf("----------\n\n")
					fmt.Printf("The error here is <%s> \n\n", string(stderr))
					return nil
				}
			}
		}
	}

	time.Sleep(120 * time.Second)
	for i := 1; i <= 200; i++ {
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", c.clusterName)
		_, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", c.clusterName)) {
				break
			}
			return err
		}

		time.Sleep(30 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBInstance) String() string {
	return fmt.Sprintf("Echo: Generating the DB instance %s ", c.clusterName)
}
