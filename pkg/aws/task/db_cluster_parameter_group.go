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
	//	"github.com/luyomo/tisample/pkg/executor"
	"strings"
)

type DBClusterParameterGroup struct {
	DBClusterParameterGroupName string `json:"DBClusterParameterGroupName"`
	DBParameterGroupFamily      string `json:"DBParameterGroupFamily"`
	Description                 string `json:"Description"`
	DBClusterParameterGroupArn  string `json:"DBClusterParameterGroupArn"`
}

type DBClusterParameterGroups struct {
	DBClusterParameterGroups []DBClusterParameterGroup `json:"DBClusterParameterGroups"`
}

type NewDBClusterParameterGroup struct {
	DBClusterParameterGroup DBClusterParameterGroup `json:"DBClusterParameterGroup"`
}

type CreateDBClusterParameterGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBClusterParameterGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-cluster-parameter-groups --db-cluster-parameter-group-name '%s'", clusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBClusterParameterGroup not found") {
			// If there is no resource, go ahead
			fmt.Printf("The DB Cluster Parameter group has not created.\n\n\n")
		} else {
			return nil
		}
	} else {
		var dbClusterParameterGroups DBClusterParameterGroups
		if err = json.Unmarshal(stdout, &dbClusterParameterGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}

		for _, dbClusterParameterGroup := range dbClusterParameterGroups.DBClusterParameterGroups {
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbClusterParameterGroup.DBClusterParameterGroupArn, *c.pexecutor, ctx)
			if existsResource == true {
				fmt.Printf("The db cluster parameter group has exists \n\n\n")
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws rds create-db-cluster-parameter-group --db-cluster-parameter-group-name %s --db-parameter-group-family aurora-mysql5.7 --description \"%s\" --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, clusterName, clusterName, clusterType, c.subClusterType)

	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var newDBClusterParameterGroup NewDBClusterParameterGroup
	if err = json.Unmarshal(stdout, &newDBClusterParameterGroup); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	command = fmt.Sprintf("aws rds modify-db-cluster-parameter-group --db-cluster-parameter-group-name %s --parameters \"ParameterName=binlog_format,ParameterValue=row,ApplyMethod=pending-reboot\"", clusterName)

	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		fmt.Printf("The error here is <%#v> \n\n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBClusterParameterGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBClusterParameterGroup) String() string {
	return fmt.Sprintf("Echo: Creating DB Cluster Parameter Group ")
}

/******************************************************************************/

type DestroyDBClusterParameterGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBClusterParameterGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-cluster-parameter-groups --db-cluster-parameter-group-name '%s'", clusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
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
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbClusterParameterGroup.DBClusterParameterGroupArn, *(c.pexecutor), ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-cluster-parameter-group --db-cluster-parameter-group-name %s", clusterName)

				stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: Destroying DB cluster parameter group")
}
