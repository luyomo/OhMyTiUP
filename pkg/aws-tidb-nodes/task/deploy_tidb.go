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
)

type DeployTiDB struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *DeployTiDB) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("Working at hte Deploy TiDB \n\n\n")
	// Filter out the instance except the terminated one.
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=16\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	fmt.Printf("The instance output is <%s>\n\n\n", string(stdout))
	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			fmt.Printf("The workstation instance ... ... ... \n\n\n")
			cntInstance++
			theInstance = instance
		}
	}
	if cntInstance > 0 {
		fmt.Printf("The instance is <%s> \n\n\n", theInstance.PublicIpAddress)
	}
	//	if len(reservations.Reservations) == 0 || len(reservations.Reservations[0].Instances) == 0 {
	//	fmt.Printf("No workstation exists")
	//	return nil
	//}

	//fmt.Printf("The workstation server ip is <%#v> \n\n\n", reservations.Reservations[0].Instances[0])

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: theInstance.PublicIpAddress, User: "admin", KeyFile: "~/.ssh/jaypingcap.pem"})
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	stdout, stderr, err = wsexecutor.Execute(ctx, `echo 'test
data ' > /tmp/test.txt`, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))

	return nil

	//workstation, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	/*

		   command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Component,Value=workstation}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.General.InstanceType, c.awsTopoConfigs.General.KeyName, clusterInfo.publicSecurityGroupId, clusterInfo.publicSubnet, c.awsTopoConfigs.General.Region, c.clusterName)
			fmt.Printf("The comamnd is <%s> \n\n\n", command)
			stdout, stderr, err = local.Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return nil
			}*/
	return nil
}

// Rollback implements the Task interface
func (c *DeployTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiDB) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
