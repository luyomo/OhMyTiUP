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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
	//	"strconv"
	//	"strings"
	"time"
)

type CreateTiDBNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreateTiDBNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	// ----- ----- TiDB
	// 1. Fetch the count of instance from config file
	// 2. If count is 0, or no TiDB, ignore it.
	if c.awsTopoConfigs.TiDB.Count == 0 {
		fmt.Printf("There is no TiDB nodes to be configured \n\n\n")
		return nil
	}
	fmt.Printf("The implementation continues\n\n\n")

	// 3. Fetch the count of instance from the instance
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=tidb\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	//	fmt.Printf("The existed nodes are <%#v> \n\n\n", reservations.Reservations)
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.TiDB.Count-existsNodes; _idx++ {
		fmt.Printf("Generating the instances <%d> \n\n\n", _idx)
		fmt.Printf("Subnets: <%s> \n\n\n", clusterInfo.privateSubnets[_idx%len(clusterInfo.privateSubnets)])
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Component,Value=tidb}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiDB.InstanceType, c.awsTopoConfigs.General.KeyName, clusterInfo.privateSecurityGroupId, clusterInfo.privateSubnets[_idx%len(clusterInfo.privateSubnets)], c.awsTopoConfigs.General.Region, c.clusterName)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		stdout, stderr, err = local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
	}
	return nil

	for i := 1; i <= 20; i++ {
		stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=tidb\"", c.clusterName), false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}

		if err = json.Unmarshal(stdout, &reservations); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}

		if len(reservations.Reservations) > 0 && len(reservations.Reservations[0].Instances) > 0 {
			fmt.Printf("The statis <%#v> \n\n\n", reservations.Reservations[0].Instances)
			if reservations.Reservations[0].Instances[0].State.Code == 16 {
				fmt.Printf("The instance has been generted.")
				break
			} else {
				fmt.Printf("The instances are <%#v> \n\n\n", reservations.Reservations[0].Instances[0])
			}
		} else {
			fmt.Printf("Failed to generate the instance")
		}
		time.Sleep(10 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTiDBNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiDBNodes) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
