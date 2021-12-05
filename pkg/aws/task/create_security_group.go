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
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
)

type CreateSecurityGroup struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
	isPrivate      bool `default:false`
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	if c.isPrivate == true {
		c.createPrivateSG(local, ctx)
	} else {
		c.createPublicSG(local, ctx)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateSecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateSecurityGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func (c *CreateSecurityGroup) createPrivateSG(executor ctxt.Executor, ctx context.Context) error {
	// Get the available zones
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName, c.clusterType, c.subClusterType)
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("security group", string(stdout)))
		return nil
	}
	if len(securityGroups.SecurityGroups) > 0 {
		c.clusterInfo.privateSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		zap.L().Info("Security group existed", zap.String("Security group", c.clusterInfo.privateSecurityGroupId))
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=private}]\"", c.clusterName, c.clusterInfo.vpcInfo.VpcId, c.clusterName, c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("create-security-group", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var securityGroup SecurityGroup
	if err = json.Unmarshal(stdout, &securityGroup); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("create-security-group", string(stdout)))
		return nil
	}
	fmt.Printf("The security group is <%s>\n\n\n", securityGroup.GroupId)
	c.clusterInfo.privateSecurityGroupId = securityGroup.GroupId
	zap.L().Info("Variable confirmation", zap.String("clusterInfo.privateSecurityGroupId", c.clusterInfo.privateSecurityGroupId))

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 1433 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 3306 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 2379 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 10080 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 8300 --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=tcp,FromPort=0,ToPort=65535,IpRanges=[{CidrIp=%s}]", c.clusterInfo.privateSecurityGroupId, c.clusterInfo.cidr)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=icmp,FromPort=-1,ToPort=-1,IpRanges=[{CidrIp=%s}]", c.clusterInfo.privateSecurityGroupId, c.clusterInfo.cidr)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}

func (c *CreateSecurityGroup) createPublicSG(executor ctxt.Executor, ctx context.Context) error {

	// Get the available zones
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-security-groups", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-security-group", string(stdout)))
		return err
	}
	if len(securityGroups.SecurityGroups) > 0 {
		c.clusterInfo.publicSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		zap.L().Info("Security group existed", zap.String("Security group", c.clusterInfo.publicSecurityGroupId))
		return err
	}

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s-public --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=public}]\"", c.clusterName, c.clusterInfo.vpcInfo.VpcId, c.clusterName, c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("create-security-group", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var securityGroup SecurityGroup
	if err = json.Unmarshal(stdout, &securityGroup); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("create-security-group", string(stdout)))
		return err
	}

	c.clusterInfo.publicSecurityGroupId = securityGroup.GroupId

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", c.clusterInfo.publicSecurityGroupId)
	zap.L().Debug("Command", zap.String("create-security-group", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return err
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=tcp,FromPort=0,ToPort=65535,IpRanges=[{CidrIp=%s}]", c.clusterInfo.publicSecurityGroupId, c.clusterInfo.cidr)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=icmp,FromPort=-1,ToPort=-1,IpRanges=[{CidrIp=%s}]", c.clusterInfo.publicSecurityGroupId, c.clusterInfo.cidr)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}
