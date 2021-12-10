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
	//	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"strings"
)

type DestroyDMSSubnetGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDMSSubnetGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws dms describe-replication-subnet-groups --filters \"Name=replication-subnet-group-id,Values=%s\"", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error err here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
		return nil
	} else {
		fmt.Printf("The data is <%s> \n\n\n", string(command))

		var dmsSubnetGroups DMSSubnetGroups
		if err = json.Unmarshal(stdout, &dmsSubnetGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		fmt.Printf("The stdout is <%s> \n\n\n", string(stdout))
		fmt.Printf("The db subnets are <%#v> \n\n\n", dmsSubnetGroups.DMSSubnetGroups)
		for _, subnet := range dmsSubnetGroups.DMSSubnetGroups {
			command = fmt.Sprintf("aws dms delete-replication-subnet-group --replication-subnet-group-identifier %s", subnet.ReplicationSubnetGroupIdentifier)
			fmt.Printf("The comamnd is <%s> \n\n\n", command)
			stdout, stderr, err = local.Execute(ctx, command, false)

			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return err
			}
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDMSSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSSubnetGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
