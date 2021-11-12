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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
	"go.uber.org/zap"
	"strings"
	"time"
)

type Vpc struct {
	CidrBlock string `json:"CidrBlock"`
	State     string `json:"State"`
	VpcId     string `json:"VpcId"`
	OwnerId   string `json:"OwnerId"`
}

type Vpcs struct {
	Vpcs []Vpc `json:"Vpcs"`
}

type CreateVpc struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

type ClusterInfo struct {
	vpcInfo                Vpc
	privateRouteTableId    string
	publicRouteTableId     string
	privateSecurityGroupId string
	publicSecurityGroupId  string
	privateSubnets         []string
	publicSubnet           string
	pcxTidb2Aurora         string
}

func (v Vpc) String() string {
	return fmt.Sprintf("Cidr: %s, State: %s, VpcId: %s, OwnerId: %s", v.CidrBlock, v.State, v.VpcId, v.OwnerId)
}

func (c ClusterInfo) String() string {
	return fmt.Sprintf("vpcInfo:[%s], privateRouteTableId:%s, publicRouteTableId:%s, privateSecurityGroupId:%s, publicSecurityGroupId:%s, privateSubnets:%s, publicSubnet:%s, pcxTidb2Aurora:%s", c.vpcInfo.String(), c.privateRouteTableId, c.publicRouteTableId, c.privateSecurityGroupId, c.publicSecurityGroupId, strings.Join(c.privateSubnets, ","), c.publicSubnet, c.pcxTidb2Aurora)
}

var clusterInfo ClusterInfo

// Execute implements the Task interface
func (c *CreateVpc) Execute(ctx context.Context) error {
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
	if len(vpcs.Vpcs) > 0 {
		clusterInfo.vpcInfo = vpcs.Vpcs[0]
		zap.L().Info("The clusterInfo.vpcInfo.vpcId is ", zap.String("VpcInfo", clusterInfo.String()))
		return nil
	}

	_, _, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc --cidr-block %s --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb}]\"", c.awsTopoConfigs.General.CIDR, c.clusterName), false)
	if err != nil {
		return nil
	}

	time.Sleep(5 * time.Second)

	zap.L().Info("Check the data before run describe-vpcs", zap.String("create-vpc", string(stdout)))
	_, _, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\"", c.clusterName), false)
	if err != nil {
		return nil
	}
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("Failed to parse the stdout", zap.String("describe-vpcs", string(stdout)))
		return nil
	}
	clusterInfo.vpcInfo = vpcs.Vpcs[0]
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
