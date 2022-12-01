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
	//"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	//	"github.com/luyomo/OhMyTiUP/pkg/executor"
	//	"go.uber.org/zap"
	//"strings"
	"time"
)

type CreateDMSInstance struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSInstance) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws dms describe-replication-instances")
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	} else {
		var replicationInstances ReplicationInstances
		if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
			return err
		}

		for _, replicationInstance := range replicationInstances.ReplicationInstances {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationInstance.ReplicationInstanceArn, *c.pexecutor, ctx)
			if existsResource == true {
				DMSInfo.ReplicationInstanceArn = replicationInstance.ReplicationInstanceArn
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws dms create-replication-instance --replication-instance-identifier %s --replication-instance-class %s --engine-version %s --replication-subnet-group-identifier %s --no-multi-az --no-publicly-accessible --replication-subnet-group-identifier %s --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, "dms.t3.medium", "3.4.6", clusterName, clusterName, clusterName, clusterType, c.subClusterType)
	stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var replicationInstanceRecord ReplicationInstanceRecord
	if err = json.Unmarshal(stdout, &replicationInstanceRecord); err != nil {
		return err
	}
	DMSInfo.ReplicationInstanceArn = replicationInstanceRecord.ReplicationInstance.ReplicationInstanceArn
	for i := 1; i <= 50; i++ {
		command = fmt.Sprintf("aws dms describe-replication-instances")
		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		} else {
			var replicationInstances ReplicationInstances
			if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
				return err
			}

			for _, replicationInstance := range replicationInstances.ReplicationInstances {
				existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationInstance.ReplicationInstanceArn, *c.pexecutor, ctx)
				if existsResource == true {
					if replicationInstance.ReplicationInstanceStatus == "available" {
						return nil
					}
				}
			}
		}

		time.Sleep(30 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSInstance) String() string {
	return fmt.Sprintf("Echo: Deploying DMS Instance ")
}

/******************************************************************************/

type DestroyDMSInstance struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyDMSInstance) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	var replicationInstanceArn string
	command := fmt.Sprintf("aws dms describe-replication-instances")
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	} else {
		var replicationInstances ReplicationInstances
		if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
			return err
		}

		for _, replicationInstance := range replicationInstances.ReplicationInstances {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, replicationInstance.ReplicationInstanceArn, *c.pexecutor, ctx)
			if existsResource == true {
				replicationInstanceArn = replicationInstance.ReplicationInstanceArn
				command = fmt.Sprintf("aws dms delete-replication-instance --replication-instance-arn %s ", replicationInstance.ReplicationInstanceArn)
				stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
				if err != nil {
					return err
				}
				break
			}
		}
	}

	var replicationInstanceRecord ReplicationInstanceRecord
	if err = json.Unmarshal(stdout, &replicationInstanceRecord); err != nil {
		return err
	}
	for i := 1; i <= 50; i++ {
		command = fmt.Sprintf("aws dms describe-replication-instances")
		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		} else {
			var replicationInstances ReplicationInstances
			if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
				return err
			}

			instanceExists := false
			for _, replicationInstance := range replicationInstances.ReplicationInstances {
				if replicationInstanceArn == replicationInstance.ReplicationInstanceArn {
					instanceExists = true
					break
				}
			}
			if instanceExists == false {
				return nil
			}
		}

		time.Sleep(30 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyDMSInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSInstance) String() string {
	return fmt.Sprintf("Echo: Destroying dms instance")
}
