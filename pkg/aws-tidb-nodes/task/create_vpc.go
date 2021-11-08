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
	"time"

	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
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

// Mkdir is used to create directory on the target host
type CreateVpc struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

type ClusterInfo struct {
	vpcInfo         Vpc
	routeTableId    string
	securityGroupId string
	subnets         []string
}

var clusterInfo ClusterInfo

// Execute implements the Task interface
func (c *CreateVpc) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	var vpcs Vpcs
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}
	if len(vpcs.Vpcs) > 0 {
		clusterInfo.vpcInfo = vpcs.Vpcs[0]
		return nil
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 create-vpc --cidr-block %s --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb}]\"", c.awsTopoConfigs.General.CIDR, c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	time.Sleep(5 * time.Second)

	fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The output data is <%s> \n\n\r\r", stdout)
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The parsed data is %#v \n\n", vpcs.Vpcs[0])
	fmt.Printf("The context data is %#v \n\n", ctx)
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
