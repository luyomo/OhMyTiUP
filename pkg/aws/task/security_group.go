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
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"go.uber.org/zap"
	"strconv"
)

type CreateSecurityGroup struct {
	pexecutor        *ctxt.Executor
	subClusterType   string
	clusterInfo      *ClusterInfo
	isPrivate        bool `default:false`
	openPortsPublic  []int
	openPortsPrivate []int
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {

	if c.isPrivate == true {
		c.createPrivateSG(*c.pexecutor, ctx)
	} else {
		c.createPublicSG(*c.pexecutor, ctx)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateSecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateSecurityGroup) String() string {
	return fmt.Sprintf("Echo: Creating security group ")
}

func (c *CreateSecurityGroup) createPrivateSG(executor ctxt.Executor, ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", clusterName, clusterType, c.subClusterType)
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

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=private}]\"", clusterName, c.clusterInfo.vpcInfo.VpcId, clusterName, clusterName, clusterType, c.subClusterType)
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

	//	for _, port := range []int{22, 1433, 2379, 2380, 3306, 4000, 8250, 8300, 9100, 10080, 20160, 20180} {
	for _, port := range c.openPortsPrivate {
		command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port %d --cidr 0.0.0.0/0", c.clusterInfo.privateSecurityGroupId, port)
		zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
		stdout, _, err = executor.Execute(ctx, command, false)
		if err != nil {
			return nil
		}
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
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Get the available zones
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", clusterName, clusterType, c.subClusterType)
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

	command = fmt.Sprintf("aws ec2 create-security-group --group-name %s-public --vpc-id %s --description %s --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Scope,Value=public}]\"", clusterName, c.clusterInfo.vpcInfo.VpcId, clusterName, clusterName, clusterType, c.subClusterType)
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

	//	for _, port := range []int{22, 80, 3000} {
	for _, port := range c.openPortsPublic {
		command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port %d --cidr 0.0.0.0/0", c.clusterInfo.publicSecurityGroupId, port)
		zap.L().Debug("Command", zap.String("authorize-security-group-ingress", command))
		stdout, _, err = executor.Execute(ctx, command, false)
		if err != nil {
			return nil
		}
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

/******************************************************************************/

type DestroySecurityGroup struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroySecurityGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", clusterName, clusterType, c.subClusterType)
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("security group", string(stdout)))
		return err
	}
	for _, sg := range securityGroups.SecurityGroups {
		command = fmt.Sprintf("aws ec2 delete-security-group --group-id %s ", sg.GroupId)
		zap.L().Debug("Command", zap.String("delete-security-group", command))
		_, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroySecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroySecurityGroup) String() string {
	return fmt.Sprintf("Echo: Destroying security group")
}

/******************************************************************************/

type ListSecurityGroup struct {
	pexecutor           *ctxt.Executor
	tableSecurityGroups *[][]string
}

// Execute implements the Task interface
func (c *ListSecurityGroup) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	command := fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" ", clusterName, clusterType)
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("security group", string(stdout)))
		return err
	}
	for _, sg := range securityGroups.SecurityGroups {
		componentName := "-"
		for _, tagItem := range sg.Tags {
			if tagItem.Key == "Type" {
				componentName = tagItem.Value
			}
		}
		for _, ipPermission := range sg.IpPermissions {
			(*c.tableSecurityGroups) = append(*c.tableSecurityGroups, []string{
				componentName,
				ipPermission.IpProtocol,
				ipPermission.IpRanges[0].CidrIp,
				strconv.Itoa(ipPermission.FromPort),
				strconv.Itoa(ipPermission.ToPort),
			})

		}
		// command = fmt.Sprintf("aws ec2 delete-security-group --group-id %s ", sg.GroupId)
		// zap.L().Debug("Command", zap.String("delete-security-group", command))
		// _, _, err = (*c.pexecutor).Execute(ctx, command, false)
		// if err != nil {
		// 	return err
		// }
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListSecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListSecurityGroup) String() string {
	return fmt.Sprintf("Echo: Listing security group")
}
