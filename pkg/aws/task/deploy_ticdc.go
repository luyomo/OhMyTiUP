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
	//	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

type DeployTiCDC struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
	clusterTable   *[][]string
}

// Execute implements the Task interface
func (c *DeployTiCDC) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	wsexecutor, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, "admin", c.clusterInfo.keyFile)
	if err != nil {
		return err
	}

	tidbClusterDetail, err := getTiDBClusterInfo(wsexecutor, ctx, clusterName, clusterType)

	auroraInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "aurora")
	fmt.Printf("The error here is <%#v> \n\n\n", err)
	fmt.Printf("\n\n\n")
	if err != nil {
		if err.Error() == "No RDB Instance found(No matched name)" {
			return nil
		}
		return err
	}
	fmt.Printf("The aurora is <%#v> \n\n\n", auroraInstance)

	for _, component := range tidbClusterDetail.Instances {
		if component.Role == "pd" && component.Status == "Up" {

			command := fmt.Sprintf(`/home/admin/.tiup/bin/tiup cdc cli changefeed list --pd http://%s:%d`, component.Host, component.Port)
			stdout, _, err := (*wsexecutor).Execute(ctx, command, false)
			if err != nil {
				return err
			}

			var cdcTasks []CDCTask
			if err = json.Unmarshal(stdout, &cdcTasks); err != nil {
				zap.L().Debug("Json unmarshal", zap.String("cdc cli changefeed list", string(stdout)))
				return err
			}
			if len(cdcTasks) == 0 {
				command := fmt.Sprintf(`/home/admin/.tiup/bin/tiup cdc cli changefeed create --pd http://%s:%d --sink-uri mysql://master:1234Abcd@%s:%d/`, component.Host, component.Port, auroraInstance.Endpoint.Address, auroraInstance.Endpoint.Port)
				stdout, _, err = (*wsexecutor).Execute(ctx, command, false)
				if err != nil {
					return err
				}
			}

			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployTiCDC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiCDC) String() string {
	return fmt.Sprintf("Echo: Deploying TiCDC")
}
