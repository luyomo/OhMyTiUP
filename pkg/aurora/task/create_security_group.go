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
	//	"github.com/luyomo/tisample/pkg/aurora/ctxt"
	"github.com/luyomo/tisample/pkg/aurora/executor"
	//	"strconv"
	//	"strings"
	//"time"
)

//type SecurityGroups struct {
//
//}

type SecurityGroups struct {
	SecurityGroups []SecurityGroup `json:"SecurityGroups"`
}
type SecurityGroup struct {
	GroupId string `json:"GroupId"`
}

type CreateSecurityGroup struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The type of local is <%T> \n\n\n", local)
	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
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
		clusterInfo.securityGroupId = securityGroups.SecurityGroups[0].GroupId
		fmt.Printf("*** *** *** Got the security group <%s> \n\n\n", clusterInfo.securityGroupId)
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-security-group --group-name tisampletest --vpc-id %s --description tisampletest --tag-specifications \"ResourceType=security-group,Tags=[{Key=Name,Value=tisampletest}]\"", clusterInfo.vpcInfo.VpcId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
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
	clusterInfo.securityGroupId = securityGroup.GroupId

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
