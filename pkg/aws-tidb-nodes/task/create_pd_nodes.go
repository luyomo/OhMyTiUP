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

type ECState struct {
	Code int    `json:"Code"`
	Name string `json:"Name"`
}

type EC2 struct {
	InstanceId       string  `json:"InstanceId"`
	State            ECState `json:"State"`
	SubnetId         string  `json:"SubnetId"`
	VpcId            string  `json:"VpcId"`
	InstanceType     string  `json:"InstanceType"`
	ImageId          string  `json:"ImageId"`
	PrivateIpAddress string  `json:"PrivateIpAddress"`
	PrivateDnsName   string  `json:"PrivateDnsName"`
}
type NewEC2 struct {
	Instances EC2 `json:"Instances"`
}

type Reservations struct {
	Reservations []EC2s `json:"Reservations"`
}

type EC2s struct {
	Instances []EC2 `json:"Instances"`
}

type CreatePDNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreatePDNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The general config info is <%#v>\n\n\n", c.awsTopoConfigs.General)
	fmt.Printf("The pd config info is <%#v>\n\n\n", c.awsTopoConfigs.PD)

	// ----- ----- PD
	// 1. Fetch the count of instance from config file
	// 2. If count is 0, or no PD, ignore it.
	if c.awsTopoConfigs.PD.Count == 0 {
		fmt.Printf("There is no PD nodes to be configured \n\n\n")
		return nil
	}
	fmt.Printf("The implementation continues\n\n\n")

	// 3. Fetch the count of instance from the instance
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=pd\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName), false)
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
	if len(reservations.Reservations) > 0 && len(reservations.Reservations[0].Instances) >= c.awsTopoConfigs.PD.Count {
		fmt.Printf("*** *** *** Got the ec2 instance <%#v> vs <%i> \n\n\n", reservations.Reservations[0].Instances, c.awsTopoConfigs.PD.Count)
		return nil
	}
	fmt.Printf("The existed nodes are <%#v> \n\n\n", reservations.Reservations)
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
		//for _, _ := range reservation.Instances {
		//	existsNodes++
		//}
	}
	return nil

	for _idx := 0; _idx < c.awsTopoConfigs.PD.Count-existsNodes; _idx++ {
		fmt.Printf("Generating the instances <%d> \n\n\n", _idx)
		fmt.Printf("Subnets: <%s> \n\n\n", clusterInfo.subnets[_idx%len(clusterInfo.subnets)])
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Component,Value=pd}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.PD.InstanceType, c.awsTopoConfigs.General.KeyName, clusterInfo.securityGroupId, clusterInfo.subnets[_idx%len(clusterInfo.subnets)], c.awsTopoConfigs.General.Region, c.clusterName)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		stdout, stderr, err = local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}
	}

	// 4. If the # of 3 is equal/more than 1, ignore

	//return nil
	// 5.1 If the # is less than 3, pick up the first two subnets
	// 5.2 if the # is equal to 3, use all these three
	// 5.3 If the # is more than 3 , go back from first

	//	fmt.Printf("The aws topo config  <%#v> \n\n\n", c.awsTopoConfigs.General)
	// Filter out the instance except the terminated one.

	// ImageId:"ami-0ac97798ccf296e02", Region:"ap-northeast-1", Name:"tisamplenodes", KeyName:"jay.pingcap"
	//command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s}]\"", c.awsTopoConfigs.General.ImageId, "t2.micro", c.awsTopoConfigs.General.KeyName, clusterInfo.securityGroupId, clusterInfo.subnets[0], c.awsTopoConfigs.General.Region, c.clusterName)
	//fmt.Printf("The comamnd is <%s> \n\n\n", command)
	//stdout, stderr, err = local.Execute(ctx, command, false)
	//if err != nil {
	//	fmt.Printf("The error here is <%#v> \n\n", err)
	//	fmt.Printf("----------\n\n")
	//	fmt.Printf("The error here is <%s> \n\n", string(stderr))
	//	return nil
	//}
	//fmt.Printf("The created instance is <%s>\n\n\n", stdout)
	for i := 1; i <= 20; i++ {
		stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=pd\"", c.clusterName), false)
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
func (c *CreatePDNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreatePDNodes) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
