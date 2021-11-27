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
	//"strings"
	//"time"
)

type CreateDMSTask struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *CreateDMSTask) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	fmt.Printf("The DMS info is <%#v> \n\n\n", DMSInfo)

	command := fmt.Sprintf("aws dms describe-replication-tasks")
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error err here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
		return nil
	} else {
		var replicationInstances ReplicationInstances
		if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", replicationInstances)
		for _, replicationInstance := range replicationInstances.ReplicationInstances {
			existsResource := ExistsDMSResource(c.clusterType, c.clusterName, replicationInstance.ReplicationInstanceArn, local, ctx)
			if existsResource == true {
				fmt.Printf("The replication instance  has exists \n\n\n")
				return nil
			}
		}
	}
	/*
			tableMapping := `{
		    "rules": [
		        {
		            "rule-type": "selection",
		            "rule-id": "1",
		            "rule-name": "1",
		            "object-locator": {
		                "schema-name": "Test",
		                "table-name": "%"
		            },
		            "rule-action": "include"
		        }
		    ]
		    }`*/
	tableMapping := `{"rules": [{"rule-type": "selection","rule-id": "1","rule-name": "1","object-locator": {"schema-name": "Test","table-name": "%"},"rule-action": "include"}]}`

	command = fmt.Sprintf("aws dms create-replication-task --replication-task-identifier %s --source-endpoint-arn %s --target-endpoint-arn %s --replication-instance-arn %s --migration-type %s --table-mappings '\"'\"'%s'\"'\"' --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, DMSInfo.SourceEndpointArn, DMSInfo.TargetEndpointArn, DMSInfo.ReplicationInstanceArn, "full-load-and-cdc", tableMapping, c.clusterName, c.clusterType)
	//	command = fmt.Sprintf("aws dms create-replication-instance --replication-instance-identifier %s --replication-instance-class %s --engine-version %s --replication-subnet-group-identifier %s --no-multi-az --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, "dms.t3.medium", "3.4.6", c.clusterName, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
