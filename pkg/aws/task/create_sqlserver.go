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
	//	"encoding/json"
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

	cnt, err := instancesExist(local, ctx, c.clusterName, c.clusterType, c.subClusterType)
	fmt.Printf("The count of ec2 instances are <%d> \n\n\n", cnt)
	fmt.Printf("The error of ec2 instances are <%#v> \n\n\n", err)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=sqlserver}]\"", c.clusterInfo.imageId, c.clusterInfo.instanceType, c.clusterInfo.keyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[0], c.clusterName, c.clusterType, c.subClusterType)

	zap.L().Debug("Command", zap.String("run-instances", command))
	_, _, err = local.Execute(ctx, command, false)
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
