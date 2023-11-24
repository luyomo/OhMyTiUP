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
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBCluster) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbClusterName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", dbClusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", dbClusterName)) {
			return err
		}
	} else {
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			return err
		}

		for _, dbCluster := range dbClusters.DBClusters {
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbCluster.DBClusterArn, *c.pexecutor, ctx)
			if existsResource == true {
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws rds create-db-cluster --db-cluster-identifier %s --engine aurora-mysql --engine-version 5.7.12 --master-username master --master-user-password 1234Abcd --db-subnet-group-name %s --db-cluster-parameter-group-name %s --vpc-security-group-ids %s --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", dbClusterName, dbClusterName, clusterName, c.clusterInfo.privateSecurityGroupId, clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var newDBCluster NewDBCluster
	if err = json.Unmarshal(stdout, &newDBCluster); err != nil {
		return err
	}

	for i := 1; i <= 30; i++ {
		command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", dbClusterName)
		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}

		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			return err
		}

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
	return fmt.Sprintf("Echo: Creating DB Cluster ")
}

/*******************************************************************************/

type DestroyDBCluster struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBCluster) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbClusterName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", dbClusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", dbClusterName)) {
			return nil
		} else {
			return err
		}
	} else {
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			return err
		}

		for _, dbCluster := range dbClusters.DBClusters {
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbCluster.DBClusterArn, *(c.pexecutor), ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-cluster --db-cluster-identifier %s --skip-final-snapshot", dbClusterName)
				stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
				if err != nil {
					return err
				}
			}
		}
	}

	for i := 1; i <= 200; i++ {
		command := fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", dbClusterName)
		_, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", dbClusterName)) {
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
	return fmt.Sprintf("Echo: Destroying DB cluster ")
}
