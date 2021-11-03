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
	"github.com/luyomo/tisample/pkg/aurora/executor"
	//"time"
)

type RegionZone struct {
	RegionName string `json:"RegionName"`
	ZoneName   string `json:"ZoneName"`
}

type AvailabilityZones struct {
	Zones []RegionZone `json:"AvailabilityZones"`
}

type Subnet struct {
	AvailabilityZone string `json:"AvailabilityZone"`
	CidrBlock        string `json:"CidrBlock"`
	State            string `json:"State"`
	SubnetId         string `json:"SubnetId"`
	VpcId            string `json:"VpcId"`
}
type Subnets struct {
	Subnets []Subnet `json:"Subnets"`
}

// Mkdir is used to create directory on the target host
type CreateNetwork struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateNetwork) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-availability-zones", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	var zones AvailabilityZones
	if err = json.Unmarshal(stdout, &zones); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	//	fmt.Printf("*** *** *** The parsed data is \n %#v \n\n", zones.Zones)
	//	fmt.Printf("The length of the zones is <%d> \n\n", len(zones.Zones))
	//fmt.Println("--------------------------- \n")

	// Get the subnets
	stdout, stderr, err = local.Execute(ctx, "aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The stdout from the local is <%s> \n\n\n", string(stdout))
	fmt.Printf("The vpc id is <%#v> \n\n\n", VpcInfo)
	for idx, zone := range zones.Zones {
		fmt.Printf("****** The zone <%d> is %s \n\n\n", idx, zone.ZoneName)
	}

	// Create the subnets for the tisampletest

	/*
		stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
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
			c.VpcId = vpcs.Vpcs[0].VpcId
			return nil
		}

		stdout, stderr, err = local.Execute(ctx, "aws ec2 create-vpc --cidr-block 172.80.0.0/16 --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=tisampletest}]\"", false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
		}

		time.Sleep(5 * time.Second)

		fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
		stdout, stderr, err = local.Execute(ctx, "aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
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
		c.VpcId = vpcs.Vpcs[0].VpcId
	*/
	return nil
}

// Rollback implements the Task interface
func (c *CreateNetwork) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNetwork) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
