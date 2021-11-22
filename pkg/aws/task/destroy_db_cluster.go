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
	"github.com/luyomo/tisample/pkg/aurora/executor"
	"strings"
	"time"
)

type DestroyDBCluster struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroyDBCluster) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", c.clusterName)) {
			fmt.Printf("The DB Cluster has not created.\n\n\n")
			return nil
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		//		fmt.Printf("The db cluster is <%#v> \n\n\n", dbClusters)
		for _, dbCluster := range dbClusters.DBClusters {
			fmt.Printf("The cluster info is <%#v> \n\n\n", dbCluster)
			existsResource := ExistsResource(c.clusterType, c.clusterName, dbCluster.DBClusterArn, local, ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-cluster --db-cluster-identifier %s --skip-final-snapshot", c.clusterName)
				fmt.Printf("The comamnd is <%s> \n\n\n", command)
				stdout, stderr, err = local.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("The error here is <%#v> \n\n", err)
					fmt.Printf("----------\n\n")
					fmt.Printf("The error here is <%s> \n\n", string(stderr))
					return err
				}

				fmt.Printf("The db cluster  has exists \n\n\n")
			}
		}
	}

	for i := 1; i <= 200; i++ {
		command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", c.clusterName)
		_, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", c.clusterName)) {
				break

			}
			return err
		}

		time.Sleep(30 * time.Second)
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBCluster) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
