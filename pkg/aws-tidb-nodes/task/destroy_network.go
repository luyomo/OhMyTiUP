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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"go.uber.org/zap"
)

type DestroyNetwork struct {
	user        string
	host        string
	clusterName string
}

// Execute implements the Task interface
func (c *DestroyNetwork) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\"", c.clusterName)
	zap.L().Debug("Command", zap.String("describe-subnets", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	for _, subnet := range subnets.Subnets {
		command := fmt.Sprintf("aws ec2 delete-subnet --subnet-id %s", subnet.SubnetId)
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
func (c *DestroyNetwork) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyNetwork) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
