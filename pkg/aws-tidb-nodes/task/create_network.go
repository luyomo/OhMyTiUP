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
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/ctxt"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
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
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

// Execute implements the Task interface
func (c *CreateNetwork) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("The type of local is <%T> \n\n\n", local)
	// Get the available zones
	//stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-availability-zones", false)
	//if err != nil {
	//	fmt.Printf("The error here is <%#v> \n\n", err)
	//	fmt.Printf("----------\n\n")
	//	fmt.Printf("The error here is <%s> \n\n", string(stderr))
	//	return nil
	//}
	////fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	//var zones AvailabilityZones
	//if err = json.Unmarshal(stdout, &zones); err != nil {
	//	fmt.Printf("*** *** The error here is %#v \n\n", err)
	//	return nil
	//}
	zones, err := getAvailableZones(local, ctx)
	if err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	c.createPrivateSubnets(local, ctx, zones)

	c.createPublicSubnets(local, ctx, zones)

	fmt.Printf("The private subnets are <%#v> \n\n\n", clusterInfo.privateSubnets)

	fmt.Printf("The public subnet is <%#v> \n\n\n", clusterInfo.publicSubnet)

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

func associateSubnet2RouteTable(subnet string, routeTableId string, executor ctxt.Executor, ctx context.Context) {
	command := fmt.Sprintf("aws ec2 associate-route-table --route-table-id %s --subnet-id %s ", routeTableId, subnet)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
	}
	fmt.Printf("The stdout is <%s>\n\n\n", stdout)
	fmt.Printf("The stderr is <%s>\n\n\n", stderr)
}

func getAvailableZones(executor ctxt.Executor, ctx context.Context) (AvailabilityZones, error) {

	// Get the available zones
	stdout, stderr, err := executor.Execute(ctx, "aws ec2 describe-availability-zones", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return AvailabilityZones{}, err
	}
	//fmt.Printf("The stdout from the local is <%s> \n\n", string(stdout))
	var zones AvailabilityZones
	if err = json.Unmarshal(stdout, &zones); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return AvailabilityZones{}, err
	}
	return zones, nil
}

func (c *CreateNetwork) createPrivateSubnets(executor ctxt.Executor, ctx context.Context, zones AvailabilityZones) error {
	// Get the subnets
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=private\"", c.clusterName), false)
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
		subnetExists := false
		for idxNet, subnet := range subnets.Subnets {
			if zone.ZoneName == subnet.AvailabilityZone {
				fmt.Printf("The subnet is <%s> and index <%d> \n\n\n", subnet.AvailabilityZone, idxNet)
				clusterInfo.privateSubnets = append(clusterInfo.privateSubnets, subnet.SubnetId)
				associateSubnet2RouteTable(subnet.SubnetId, clusterInfo.privateRouteTableId, executor, ctx)
				subnetExists = true
			}
		}
		if subnetExists == true {
			continue
		}

		command := fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=private}]\"", getNextCidr(clusterInfo.vpcInfo.CidrBlock, idx+1), clusterInfo.vpcInfo.VpcId, zone.ZoneName, c.clusterName)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		sub_stdout, sub_stderr, sub_err := executor.Execute(ctx, command, false)
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
		associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.privateRouteTableId, executor, ctx)
		clusterInfo.privateSubnets = append(clusterInfo.privateSubnets, newSubnet.Subnet.SubnetId)
	}

	return nil
}

func (c *CreateNetwork) createPublicSubnets(executor ctxt.Executor, ctx context.Context, zones AvailabilityZones) error {
	// Get the subnets
	stdout, stderr, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=public\"", c.clusterName), false)
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

	if len(subnets.Subnets) > 0 {
		clusterInfo.publicSubnet = subnets.Subnets[0].SubnetId
		fmt.Printf("The public subnet has been created. ")
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-subnet --cidr-block %s --vpc-id %s --availability-zone=%s --tag-specifications \"ResourceType=subnet,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Scope,Value=public}]\"", getNextCidr(clusterInfo.vpcInfo.CidrBlock, 10+1), clusterInfo.vpcInfo.VpcId, zones.Zones[0].ZoneName, c.clusterName)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	sub_stdout, sub_stderr, sub_err := executor.Execute(ctx, command, false)
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
	associateSubnet2RouteTable(newSubnet.Subnet.SubnetId, clusterInfo.publicRouteTableId, executor, ctx)
	clusterInfo.publicSubnet = newSubnet.Subnet.SubnetId

	return nil
}
