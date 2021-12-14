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
	//	"go.uber.org/zap"
	"strings"
	//"time"
)

type DestroyDMSTask struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDMSTask) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	command := fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("No Tasks found matching provided filters")) {
			fmt.Printf("The Replication task has not created.\n\n\n")
			return nil
		} else {
			fmt.Printf("ERRORS: describe-replication-tasks  <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var replicationTasks ReplicationTasks
		if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
			fmt.Printf("ERROR: describe-replication-tasks json parsing %#v \n\n", err)
			return err
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", replicationTasks)
		for _, replicationTask := range replicationTasks.ReplicationTasks {
			existsResource := ExistsDMSResource(c.clusterType, c.subClusterType, c.clusterName, replicationTask.ReplicationTaskArn, local, ctx)
			if existsResource == true {
				if replicationTask.Status == "running" {
					command = fmt.Sprintf("aws dms stop-replication-task --replication-task-arn %s", replicationTask.ReplicationTaskArn)
					fmt.Printf("The comamnd is <%s> \n\n\n", command)
					stdout, stderr, err = local.Execute(ctx, command, false)
					if err != nil {
						fmt.Printf("ERROR: stop-replicaion-task-arn <%s> \n\n\n", string(stderr))
						return err
					}
				}

				command = fmt.Sprintf("aws dms delete-replication-task --replication-task-arn %s", replicationTask.ReplicationTaskArn)
				fmt.Printf("The comamnd is <%s> \n\n\n", command)
				stdout, stderr, err = local.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("ERROR: destroy_dms_task delete-replixarion-task <%s> \n\n\n", string(stderr))
					return err
				}
				return nil
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyDMSTask) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSTask) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
