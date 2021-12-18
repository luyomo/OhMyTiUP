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
	//	"strings"
	"time"
)

type CreateVpc struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateVpc) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	stdout, _, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", c.clusterName, c.clusterType, c.subClusterType), false)
	if err != nil {
		return err
	}
	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return err
	}
	if len(vpcs.Vpcs) > 0 {
		c.clusterInfo.vpcInfo = vpcs.Vpcs[0]
		zap.L().Info("The clusterInfo.vpcInfo.vpcId is ", zap.String("VpcInfo", c.clusterInfo.String()))
		return err
	}

	_, _, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc --cidr-block %s --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s}]\"", c.clusterInfo.cidr, c.clusterName, c.clusterType, c.subClusterType), false)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)

	zap.L().Info("Check the data before run describe-vpcs", zap.String("create-vpc", string(stdout)))
	stdout, _, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\"  ", c.clusterName, c.clusterType, c.subClusterType), false)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("Failed to parse the stdout", zap.String("describe-vpcs", string(stdout)))
		return err
	}
	c.clusterInfo.vpcInfo = vpcs.Vpcs[0]
	return nil
}

// Rollback implements the Task interface
func (c *CreateVpc) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateVpc) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type DestroyVpc struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyVpc) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", c.clusterName, c.clusterType, c.subClusterType), false)
	if err != nil {
		fmt.Printf("ERROR describe-vpcs <%s>", string(stderr))
		return err
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
			fmt.Printf("ERROR describe-vpc <%s>", string(stderr))
			return err
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
