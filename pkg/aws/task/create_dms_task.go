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
	"time"
)

type CreateDMSTask struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSTask) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	command := fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("No Tasks found matching provided filters")) {
			fmt.Printf("The Replication task has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var replicationTasks ReplicationTasks
		if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", replicationTasks)
		for _, replicationTask := range replicationTasks.ReplicationTasks {
			existsResource := ExistsDMSResource(c.clusterType, c.subClusterType, c.clusterName, replicationTask.ReplicationTaskArn, local, ctx)
			if existsResource == true {
				fmt.Printf("The replication instance  has exists \n\n\n")
				return nil
			}
		}
	}

	tableMapping := `{"rules": [{"rule-type": "selection","rule-id": "1","rule-name": "1","object-locator": {"schema-name": "Test","table-name": "%"},"rule-action": "include"}]}`

	command = fmt.Sprintf("aws dms create-replication-task --replication-task-identifier %s --source-endpoint-arn %s --target-endpoint-arn %s --replication-instance-arn %s --migration-type %s --table-mappings '\"'\"'%s'\"'\"' --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, DMSInfo.SourceEndpointArn, DMSInfo.TargetEndpointArn, DMSInfo.ReplicationInstanceArn, "full-load-and-cdc", tableMapping, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}
	var replicationTaskRecord ReplicationTaskRecord
	if err = json.Unmarshal(stdout, &replicationTaskRecord); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The parsed recotd is <%#v> \n\n\n", replicationTaskRecord)

	taskIsReady := false
	for i := 1; i <= 50; i++ {
		command = fmt.Sprintf("aws dms describe-replication-tasks --filters Name=replication-task-id,Values=%s", c.clusterName)

		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return err

		} else {
			var replicationTasks ReplicationTasks
			if err = json.Unmarshal(stdout, &replicationTasks); err != nil {
				fmt.Printf("*** *** The error here is %#v \n\n", err)
				return err
			}
			fmt.Printf("The db cluster is <%#v> \n\n\n", replicationTasks)
			for _, replicationTask := range replicationTasks.ReplicationTasks {
				existsResource := ExistsDMSResource(c.clusterType, c.subClusterType, c.clusterName, replicationTask.ReplicationTaskArn, local, ctx)
				if existsResource == true {
					if replicationTask.Status == "ready" {
						fmt.Printf("The task becomes ready \n\n\n")
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
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}

	fmt.Printf("To start the task  \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSTask) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSTask) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
