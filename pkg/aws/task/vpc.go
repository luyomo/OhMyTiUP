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
	"errors"
	"fmt"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
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
	if err != nil {
		zap.L().Debug("Failed to create the executor ", zap.Error(err))
		return err
	}

	vpcInfo, error := getVPCInfo(local, ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType, subClusterType: c.subClusterType})
	if error == nil {
		zap.L().Info("Fetched VPC Info", zap.String("VPC Info", vpcInfo.String()))
		c.clusterInfo.vpcInfo = *vpcInfo
		return errors.New("Debug")
	}
	if error.Error() != "No VPC found" {
		zap.L().Debug("Failed to fetch vpc info ", zap.Error(err))
		return err
	}

	_, _, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc --cidr-block %s --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s}]\"", c.clusterInfo.cidr, c.clusterName, c.clusterType, c.subClusterType), false)
	if err != nil {
		zap.L().Error("Failed to create vpc. VPCInfo: ", zap.String("VpcInfo", c.clusterInfo.String()))
		return err
	}

	time.Sleep(5 * time.Second)

	vpcInfo, error = getVPCInfo(local, ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType, subClusterType: c.subClusterType})
	if error == nil {
		zap.L().Info("Fetched VPC Info", zap.String("VPC Info", vpcInfo.String()))
		c.clusterInfo.vpcInfo = *vpcInfo
		return nil
	}

	return errors.New("Failed to create vpc")
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

/*
   Description: Destroy the VPC if it does not exists.
*/
func (c *DestroyVpc) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		zap.L().Debug("Failed to create the executor ", zap.Error(err))
		return err
	}

	// Fetch the vpc info.
	//  1. Return if no vpc is found
	//  2. Return error if it fails
	vpcInfo, error := getVPCInfo(local, ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType, subClusterType: c.subClusterType})
	if error.Error() == "No VPC found" {
		return nil
	}
	if error != nil {
		zap.L().Debug("Failed to fetch vpc info ", zap.Error(err))
		return err
	}

	// Delete the specified vpc
	command := fmt.Sprintf("aws ec2 delete-vpc --vpc-id %s", (*vpcInfo).VpcId)
	_, _, err = local.Execute(ctx, command, false)
	if err != nil {
		zap.L().Debug("Failed to delete vpc info ", zap.Error(err))
		return err
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
