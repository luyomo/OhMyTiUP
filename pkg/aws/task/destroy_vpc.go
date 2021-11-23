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
	//	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
	//	"strings"
	//	"time"
)

type DestroyVpc struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroyVpc) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	stdout, _, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", c.clusterName), false)
	if err != nil {
		return nil
	}
	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil
	}
	for _, vpc := range vpcs.Vpcs {
		fmt.Printf("The data here is <%#v> \n\n\n", vpc)
		command := fmt.Sprintf("aws ec2 delete-vpc --vpc-id %s", vpc.VpcId)
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
func (c *DestroyVpc) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVpc) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
