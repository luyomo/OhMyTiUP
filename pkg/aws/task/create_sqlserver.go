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
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
)

type CreateMS struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateMS) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}
	fmt.Printf(" *** *** *** Running inside the create sql server \n\n\n")
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=sqlserver\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
	}
	for _, reservation := range reservations.Reservations {
		for _, _ = range reservation.Instances {
			return nil
		}
	}

	command = fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=sqlserver}]\"", "ami-01d445a80199e19cc", c.awsTopoConfigs.General.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[0], c.awsTopoConfigs.General.Region, c.clusterName, c.clusterType)

	zap.L().Debug("Command", zap.String("run-instances", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateMS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateMS) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
