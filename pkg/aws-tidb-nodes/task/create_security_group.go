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
	//	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/ctxt"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/ctxt"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
)

type SecurityGroups struct {
	SecurityGroups []SecurityGroup `json:"SecurityGroups"`
}
type SecurityGroup struct {
	GroupId string `json:"GroupId"`
}

type CreateSecurityGroup struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		fmt.Printf("Failed to generate the executor ")
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
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	if len(securityGroups.SecurityGroups) > 0 {
		clusterInfo.privateSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		fmt.Printf("*** *** *** Got the security group <%s> \n\n\n", clusterInfo.privateSecurityGroupId)
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-security-group --group-name %s --vpc-id %s --description tisamplenodes --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=private}]\"", c.clusterName, clusterInfo.vpcInfo.VpcId, c.clusterName)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The security group is <%s>\n\n\n", stdout)
	var securityGroup SecurityGroup
	if err = json.Unmarshal(stdout, &securityGroup); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n\n", err)
		return nil
	}
	fmt.Printf("The security group is <%s>\n\n\n", securityGroup.GroupId)
	clusterInfo.privateSecurityGroupId = securityGroup.GroupId

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", clusterInfo.privateSecurityGroupId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --cidr %s", clusterInfo.privateSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol icmp --cidr %s", clusterInfo.privateSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	return nil
}

func (c *CreateSecurityGroup) createPublicSG(executor ctxt.Executor, ctx context.Context) error {

	// Get the available zones
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	if len(securityGroups.SecurityGroups) > 0 {
		clusterInfo.publicSecurityGroupId = securityGroups.SecurityGroups[0].GroupId
		fmt.Printf("*** *** *** Got the security group <%s> \n\n\n", clusterInfo.publicSecurityGroupId)
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-security-group --group-name %s-public --vpc-id %s --description tisamplenodes --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=public}]\"", c.clusterName, clusterInfo.vpcInfo.VpcId, c.clusterName)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The security group is <%s>\n\n\n", stdout)
	var securityGroup SecurityGroup
	if err = json.Unmarshal(stdout, &securityGroup); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n\n", err)
		return nil
	}
	fmt.Printf("The security group is <%s>\n\n\n", securityGroup.GroupId)
	clusterInfo.publicSecurityGroupId = securityGroup.GroupId

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --port 22 --cidr 0.0.0.0/0", clusterInfo.publicSecurityGroupId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol tcp --cidr %s", clusterInfo.publicSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	command = fmt.Sprintf("aws ec2 authorize-security-group-ingress --group-id %s --protocol icmp --cidr %s", clusterInfo.publicSecurityGroupId, c.awsTopoConfigs.General.CIDR)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	return nil
}
