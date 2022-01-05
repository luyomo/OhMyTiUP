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

type DBParameterGroup struct {
	DBParameterGroupName   string `json:"DBParameterGroupName"`
	DBParameterGroupFamily string `json:"DBParameterGroupFamily"`
	Description            string `json:"Description"`
	DBParameterGroupArn    string `json:"DBParameterGroupArn"`
}

type DBParameterGroups struct {
	DBParameterGroups []DBParameterGroup `json:"DBParameterGroups"`
}

type NewDBParameterGroup struct {
	DBParameterGroup DBParameterGroup `json:"DBParameterGroup"`
}

type CreateDBParameterGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	groupFamily    string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBParameterGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbParameterName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-parameter-groups --db-parameter-group-name '%s'", dbParameterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), "DBParameterGroup not found") {
			return err
		}
	} else {
		var dbParameterGroups DBParameterGroups
		if err = json.Unmarshal(stdout, &dbParameterGroups); err != nil {
			return err
		}

		for _, dbParameterGroup := range dbParameterGroups.DBParameterGroups {

			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbParameterGroup.DBParameterGroupArn, *c.pexecutor, ctx)
			if existsResource == true {
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws rds create-db-parameter-group --db-parameter-group-name %s --db-parameter-group-family %s --description \"%s\" --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", dbParameterName, c.groupFamily, clusterName, clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var newDBParameterGroup NewDBParameterGroup
	if err = json.Unmarshal(stdout, &newDBParameterGroup); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBParameterGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBParameterGroup) String() string {
	return fmt.Sprintf("Echo: Creating DB Parameter Group ")
}

/******************************************************************************/

type DestroyDBParameterGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBParameterGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	dbParameterName := fmt.Sprintf("%s-%s", clusterName, c.subClusterType)
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-parameter-groups --db-parameter-group-name '%s'", dbParameterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), "DBParameterGroup not found") {
			return err
		}
		return nil
	} else {
		var dbParameterGroups DBParameterGroups
		if err = json.Unmarshal(stdout, &dbParameterGroups); err != nil {
			return err
		}
		for _, dbParameterGroup := range dbParameterGroups.DBParameterGroups {
			existsResource := ExistsResource(clusterType, c.subClusterType, clusterName, dbParameterGroup.DBParameterGroupArn, *(c.pexecutor), ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws rds delete-db-parameter-group --db-parameter-group-name %s", dbParameterName)
				stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
				if err != nil {
					return err
				}
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
	return fmt.Sprintf("Echo: Destrying DB Parameter group")
}
