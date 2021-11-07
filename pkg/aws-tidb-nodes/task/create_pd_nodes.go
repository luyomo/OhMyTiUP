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
	//	"strconv"
	//	"strings"
	"time"
)

//type SecurityGroups struct {
//
//}

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
	user string
	host string
}

// Execute implements the Task interface
func (c *CreatePDNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The type of local is <%T> \n\n\n", local)
	// Filter out the instance except the terminated one.
	stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisamplenodes\" \"Name=instance-state-code,Values=0,16,32,64,80\"", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	fmt.Printf("The instance output is <%s>", string(stdout))
	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	if len(reservations.Reservations) > 0 && len(reservations.Reservations[0].Instances) > 0 {
		fmt.Printf("*** *** *** Got the ec2 instance <%#v> \n\n\n", reservations.Reservations[0].Instances)
		return nil
	}

	command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s}]\"", "ami-0ac97798ccf296e02", "t2.micro", "jay.pingcap", clusterInfo.securityGroupId, clusterInfo.subnets[0], "ap-northeast-1", "tisamplenodes")
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The created instance is <%s>\n\n\n", stdout)
	for i := 1; i <= 10; i++ {
		stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisamplenodes\" \"Name=instance-state-code,Values=0,16\"", false)
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
