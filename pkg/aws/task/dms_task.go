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
	"errors"
	"fmt"
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"go.uber.org/zap"
	"strings"
	"time"
)

type CreateDMSTask struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSTask) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", clusterName)
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), fmt.Sprintf("No Tasks found matching provided filters")) {
			return err
		}
	} else {
		var replicationTasks ReplicationTasks
		if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
			return err
		}
		for _, replicationTask := range replicationTasks.ReplicationTasks {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationTask.ReplicationTaskArn, *c.pexecutor, ctx)
			if existsResource == true {
				return nil
			}
		}
	}

	tableMapping := `{"rules": [{"rule-type": "selection","rule-id": "1","rule-name": "1","object-locator": {"schema-name": "cdc_test","table-name": "%"},"rule-action": "include"}]}`

	command = fmt.Sprintf("aws dms create-replication-task --replication-task-identifier %s --source-endpoint-arn %s --target-endpoint-arn %s --replication-instance-arn %s --migration-type %s --table-mappings '\"'\"'%s'\"'\"' --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, DMSInfo.SourceEndpointArn, DMSInfo.TargetEndpointArn, DMSInfo.ReplicationInstanceArn, "full-load-and-cdc", tableMapping, clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var replicationTaskRecord ReplicationTaskRecord
	if err = json.Unmarshal(stdout, &replicationTaskRecord); err != nil {
		return nil
	}

	taskIsReady := false
	for i := 1; i <= 50; i++ {
		command = fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", clusterName)

		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err

		} else {
			var replicationTasks ReplicationTasks
			if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
				return err
			}

			for _, replicationTask := range replicationTasks.ReplicationTasks {
				existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationTask.ReplicationTaskArn, *c.pexecutor, ctx)
				if existsResource == true {
					if replicationTask.Status == "ready" {
						taskIsReady = true
						break
					}
				}
			}
			if taskIsReady == true {
				break
			}
		}

		time.Sleep(30 * time.Second)
	}
	command = fmt.Sprintf("aws dms start-replication-task --replication-task-arn %s --start-replication-task-type start-replication", replicationTaskRecord.ReplicationTask.ReplicationTaskArn)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSTask) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSTask) String() string {
	return fmt.Sprintf("Echo: Creating DMS Task")
}

/******************************************************************************/

type DestroyDMSTask struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDMSTask) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	for i := 1; i <= 200; i++ {
		command := fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", clusterName)
		stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			if strings.Contains(string(stderr), fmt.Sprintf("No Tasks found matching provided filters")) {
				return nil
			} else {
				return err
			}
		} else {
			var replicationTasks ReplicationTasks
			if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
				return err
			}

			if len(replicationTasks.ReplicationTasks) == 0 {
				return nil
			}
			for _, replicationTask := range replicationTasks.ReplicationTasks {
				existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationTask.ReplicationTaskArn, (*c.pexecutor), ctx)
				if existsResource == true {
					if replicationTask.Status == "running" {
						command = fmt.Sprintf("aws dms stop-replication-task --replication-task-arn %s", replicationTask.ReplicationTaskArn)
						stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
						if err != nil {
							return err
						}
					}
					if replicationTask.Status == "deleting" {
						continue
					}

					command = fmt.Sprintf("aws dms delete-replication-task --replication-task-arn %s", replicationTask.ReplicationTaskArn)
					stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
					if err != nil {
						return err
					}
				}
			}

		}
		time.Sleep(30 * time.Second)
	}

	return errors.New("Failed to stop the delete-replication-task")
}

// Rollback implements the Task interface
func (c *DestroyDMSTask) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSTask) String() string {
	return fmt.Sprintf("Echo: destroying dm task ")
}
