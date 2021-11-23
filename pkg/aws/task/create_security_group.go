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
	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
	"strings"
)

type SecurityGroups struct {
	SecurityGroups []SecurityGroup `json:"SecurityGroups"`
}
type SecurityGroup struct {
	GroupId string `json:"GroupId"`
}

func (s SecurityGroup) String() string {
	return fmt.Sprintf(s.GroupId)
}

func (i SecurityGroups) String() string {
	var res []string
	for _, sg := range i.SecurityGroups {
		res = append(res, sg.String())
	}
	return strings.Join(res, ",")
}

type CreateSecurityGroup struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	c.createPrivateSG(local, ctx)

	c.createPublicSG(local, ctx)

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
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName, c.clusterType)
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
		clusterInfo.privateSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		zap.L().Info("Security group existed", zap.String("Security group", clusterInfo.privateSecurityGroupId))
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=private}]\"", c.clusterName, clusterInfo.vpcInfo.VpcId, c.clusterName, c.clusterName, c.clusterType)
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
	clusterInfo.privateSecurityGroupId = securityGroup.GroupId
	zap.L().Info("Variable confirmation", zap.String("clusterInfo.privateSecurityGroupId", clusterInfo.privateSecurityGroupId))

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", clusterInfo.privateSecurityGroupId)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=tcp,FromPort=0,ToPort=65535,IpRanges=[{CidrIp=%s}]", clusterInfo.privateSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=icmp,FromPort=-1,ToPort=-1,IpRanges=[{CidrIp=%s}]", clusterInfo.privateSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}

func (c *CreateSecurityGroup) createPublicSG(executor ctxt.Executor, ctx context.Context) error {

	// Get the available zones
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName, c.clusterType)
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
		clusterInfo.publicSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		zap.L().Info("Security group existed", zap.String("Security group", clusterInfo.publicSecurityGroupId))
		return err
	}

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s-public --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=public}]\"", c.clusterName, clusterInfo.vpcInfo.VpcId, c.clusterName, c.clusterName, c.clusterType)
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

	clusterInfo.publicSecurityGroupId = securityGroup.GroupId

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", clusterInfo.publicSecurityGroupId)
	zap.L().Debug("Command", zap.String("create-security-group", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return err
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=tcp,FromPort=0,ToPort=65535,IpRanges=[{CidrIp=%s}]", clusterInfo.publicSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --ip-permissions IpProtocol=icmp,FromPort=-1,ToPort=-1,IpRanges=[{CidrIp=%s}]", clusterInfo.publicSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
	stdout, _, err = executor.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}
