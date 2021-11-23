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
	"time"
)

type DBCluster struct {
	AllocatedStorage    int    `json:"AllocatedStorage"`
	DBClusterIdentifier string `json:"DBClusterIdentifier"`
	Status              string `json:"Status"`
	DBClusterArn        string `json:"DBClusterArn"`
}

type NewDBCluster struct {
	DBCluster DBCluster `json:"DBCluster"`
}

type DBClusters struct {
	DBClusters []DBCluster `json:"DBClusters"`
}

type CreateDBCluster struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *CreateDBCluster) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", c.clusterName)) {
			fmt.Printf("The DB Cluster has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", dbClusters)
		for _, dbCluster := range dbClusters.DBClusters {
			fmt.Printf("The cluster info is <%#v> \n\n\n", dbCluster)
			existsResource := ExistsResource(c.clusterType, c.clusterName, dbCluster.DBClusterArn, local, ctx)
			if existsResource == true {
				fmt.Printf("The db cluster  has exists \n\n\n")
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws rds create-db-cluster --db-cluster-identifier %s --engine aurora-mysql --engine-version 5.7.12 --master-username master --master-user-password 1234Abcd --db-subnet-group-name %s --db-cluster-parameter-group-name %s --vpc-security-group-ids %s --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, c.clusterName, c.clusterName, clusterInfo.privateSecurityGroupId, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var newDBCluster NewDBCluster
	if err = json.Unmarshal(stdout, &newDBCluster); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The db cluster is <%#v>\n\n\n", newDBCluster)

	for i := 1; i <= 30; i++ {
		command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", c.clusterName)
		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
		//fmt.Printf("The db cluster is <%#v>\n\n\n", string(stdout))
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v>\n\n\n", dbClusters)
		if dbClusters.DBClusters[0].Status == "available" {
			break
		}
		time.Sleep(10 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBCluster) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
