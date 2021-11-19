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
	"github.com/luyomo/tisample/pkg/aws/executor"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
)

// Mkdir is used to create directory on the target host
type DestroyRouteTable struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *DestroyRouteTable) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	stdout, _, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", c.clusterName, c.clusterType), false)
	if err != nil {
		return nil
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return nil
	}

	for _, routeTable := range routeTables.RouteTables {
		command := fmt.Sprintf("aws ec2 delete-route-table --route-table-id %s", routeTable.RouteTableId)
		stdout, _, err = local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyRouteTable) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyRouteTable) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
