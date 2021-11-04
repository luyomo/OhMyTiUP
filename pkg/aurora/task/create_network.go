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
	"strconv"
	"strings"
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

type SubnetResult struct {
	Subnet Subnet `json:"Subnet"`
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
	//fmt.Printf("The stdout from the local is <%s> \n\n\n", string(stdout))
	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	for idx, zone := range zones.Zones {
		fmt.Printf("********** ****** The zone <%d> is %s \n", idx, zone.ZoneName)
		fmt.Printf("cidr block : <%s> \n", getNextCidr(VpcInfo.CidrBlock, idx+1))
		fmt.Printf("vpc id is <%s> \n", VpcInfo.VpcId)
		fmt.Printf("The available zone is: <%s> \n\n\n", zone.ZoneName)
		subnetExists := false
		for idxNet, subnet := range subnets.Subnets {
			if zone.ZoneName == subnet.AvailabilityZone {
				fmt.Printf("The subnet is <%s> and index <%d> \n\n\n", subnet.AvailabilityZone, idxNet)
				subnetExists = true
			}
		}
		if subnetExists == true {
			continue
		}

		command := fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=tisampletest}]\"", getNextCidr(VpcInfo.CidrBlock, idx+1), VpcInfo.VpcId, zone.ZoneName)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		sub_stdout, sub_stderr, sub_err := local.Execute(ctx, command, false)
		if sub_err != nil {
			fmt.Printf("The error here is <%#v> \n\n", sub_err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(sub_stderr))
			return nil
		}
		var newSubnet SubnetResult
		if err = json.Unmarshal(sub_stdout, &newSubnet); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n\n", err)
			return nil
		}
		//fmt.Printf("The stdout from the subnett preparation: %s \n\n\n", sub_stdout)
		fmt.Printf("The stdout from the subnett preparation: %s and %s \n\n\n", newSubnet.Subnet.State, newSubnet.Subnet.CidrBlock)
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

func getNextCidr(cidr string, idx int) string {
	ip := strings.Split(cidr, "/")[0]
	ipSegs := strings.Split(ip, ".")
	//	maskLen := strings.Split(cidr, "/")[1]
	return ipSegs[0] + "." + ipSegs[1] + "." + strconv.Itoa(idx) + ".0/24"
}
