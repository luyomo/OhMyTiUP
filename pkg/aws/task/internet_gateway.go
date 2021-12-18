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

// Mkdir is used to create directory on the target host
type CreateInternetGateway struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateInternetGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-internet-gateways --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Clustere\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-internet-gateways", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var internetGateways InternetGateways
	if err = json.Unmarshal(stdout, &internetGateways); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	if len(internetGateways.InternetGateways) > 0 {
		zap.L().Info("Internetgateways existed", zap.String("Internetgateway", internetGateways.String()))
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-internet-gateway --tag-specifications \"ResourceType=internet-gateway,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s}]\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("create-internet-gateway", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var newInternetGateway NewInternetGateway
	if err = json.Unmarshal(stdout, &newInternetGateway); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("new internet gateway", string(stdout)))
		return nil
	}

	zap.L().Debug("New Internet gateway", zap.String("newInternetGateway", newInternetGateway.String()))
	//	fmt.Printf("The stdout from the internet gateway preparation: %#v \n\n\n", newInternetGateway)
	command = fmt.Sprintf("aws ec2 attach-internet-gateway --internet-gateway-id %s --vpc-id %s", newInternetGateway.InternetGateway.InternetGatewayId, c.clusterInfo.vpcInfo.VpcId)
	zap.L().Debug("Command", zap.String("create-internet-gateway", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block 0.0.0.0/0 --gateway-id %s", c.clusterInfo.publicRouteTableId, newInternetGateway.InternetGateway.InternetGatewayId)
	zap.L().Debug("Command", zap.String("create-route", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateInternetGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

// Mkdir is used to create directory on the target host
type DestroyInternetGateway struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyInternetGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-internet-gateways --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-internet-gateways", command))
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error in the DestroyInternetGateway describe-internet-gateways <%s> \n\n\n", string(stderr))
		return err
	}

	var internetGateways InternetGateways
	if err = json.Unmarshal(stdout, &internetGateways); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	for _, internetGateway := range internetGateways.InternetGateways {
		fmt.Printf("The internet gateway <%#v> \n\n\n", internetGateway)
		for _, attachment := range internetGateway.Attachments {
			command = fmt.Sprintf("aws ec2 detach-internet-gateway --internet-gateway-id %s --vpc-id %s", internetGateway.InternetGatewayId, attachment.VpcId)
			zap.L().Debug("Command", zap.String("detach-internet-gateway", command))
			_, _, err := local.Execute(ctx, command, false)
			if err != nil {
				return nil
			}
		}

		command = fmt.Sprintf("aws ec2 delete-internet-gateway --internet-gateway-id %s", internetGateway.InternetGatewayId)
		zap.L().Debug("Command", zap.String("delete-internet-gateway", command))
		stdout, _, err = local.Execute(ctx, command, false)
		if err != nil {
			return nil
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyInternetGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
