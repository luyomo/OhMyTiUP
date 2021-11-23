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
	"go.uber.org/zap"
)

type DestroySecurityGroup struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroySecurityGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\"", c.clusterName, c.clusterType)
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("security group", string(stdout)))
		return nil
	}
	for _, sg := range securityGroups.SecurityGroups {
		fmt.Printf("The data is <%#v> \n\n\n", sg.GroupId)
		command = fmt.Sprintf("aws ec2 delete-security-group --group-id %s ", sg.GroupId)
		zap.L().Debug("Command", zap.String("delete-security-group", command))
		_, _, err = local.Execute(ctx, command, false)
		if err != nil {
			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroySecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroySecurityGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
