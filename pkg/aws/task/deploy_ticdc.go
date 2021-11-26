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

type DeployTiCDC struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DeployTiCDC) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=16\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
	}

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			cntInstance++
			theInstance = instance
		}
	}

	command = fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: theInstance.PublicIpAddress, User: "admin", KeyFile: "~/.ssh/jaypingcap.pem"})
	if err != nil {
		return nil
	}

	stdout, stderr, err := wsexecutor.Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster display %s --format json `, c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		return nil
	}

	var tidbClusterDetail TiDBClusterDetail
	if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return nil
	}
	for _, component := range tidbClusterDetail.Instances {
		if component.Role == "pd" && component.Status == "Up" {

			command := fmt.Sprintf(`/home/admin/.tiup/bin/tiup cdc cli changefeed list --pd http://%s:%d`, component.Host, component.Port)
			stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
				return err
			}

			var cdcTasks []CDCTask
			fmt.Printf("originakl string <%s> \n\n\n", string(stdout))
			if err = json.Unmarshal(stdout, &cdcTasks); err != nil {
				zap.L().Debug("Json unmarshal", zap.String("cdc cli changefeed list", string(stdout)))
				return nil
			}
			fmt.Printf("The cdc tasks are <%#v> \n\n\n", cdcTasks)
			if len(cdcTasks) == 0 {
				command := fmt.Sprintf(`/home/admin/.tiup/bin/tiup cdc cli changefeed create --pd http://%s:%d --sink-uri mysql://master:1234Abcd@%s:%d/`, component.Host, component.Port, auroraConnInfo.Address, auroraConnInfo.Port)
				stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
				if err != nil {
					fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
					return err
				}
			}

			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployTiCDC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiCDC) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
