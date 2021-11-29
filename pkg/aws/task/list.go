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
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
	"sort"
	//"time"
)

type ARNComponent struct {
	ComponentType string
	ComponentName string
	ComponentID   string
	ImageID       string
	InstanceName  string
	KeyName       string
	State         string
	CIDR          string
	Region        string
	Zone          string
}

type ByComponentType []ARNComponent

func (a ByComponentType) Len() int           { return len(a) }
func (a ByComponentType) Less(i, j int) bool { return a[i].ComponentType < a[j].ComponentType }
func (a ByComponentType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type List struct {
	User          string
	Host          string
	ArnComponents []ARNComponent
}

// Execute implements the Task interface
func (c *List) Execute(ctx context.Context, clusterName, clusterType string) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.User})

	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", clusterName, clusterType), false)
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

	for _, vpc := range vpcs.Vpcs {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"VPC",
			clusterName,
			vpc.VpcId,
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
		})
	}

	// Get the route table
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	for _, routeTable := range routeTables.RouteTables {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"Route Table",
			clusterName,
			routeTable.RouteTableId,
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
		})
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
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

	for _, subnet := range subnets.Subnets {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"Subnet",
			clusterName,
			subnet.SubnetId,
			"-",
			"-",
			"-",
			subnet.State,
			subnet.CidrBlock,
			subnet.AvailabilityZone,
			"-",
		})
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
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

	for _, securityGroup := range securityGroups.SecurityGroups {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"Security Group",
			clusterName,
			securityGroup.GroupId,
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
		})
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=status-code,Values=failed,expired,provisioning,active,rejected\"", clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var vpcConnections VpcConnections
	if err = json.Unmarshal(stdout, &vpcConnections); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}

	for _, vpcPeering := range vpcConnections.VpcPeeringConnections {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"VPC Peering",
			clusterName,
			vpcPeering.VpcPeeringConnectionId,
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
		})
	}

	//state := ""
	for _, pcx := range vpcConnections.VpcPeeringConnections {
		if pcx.VpcStatus.Code == "active" {
			//state = "active"
			//			clusterInfo.pcxTidb2Aurora = pcx.VpcPeeringConnectionId
		}
	}

	// Internet gateway info
	command := fmt.Sprintf("aws ec2 describe-internet-gateways --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", clusterName, clusterType)
	zap.L().Debug("Command", zap.String("describe-internet-gateways", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var internetGateways InternetGateways
	if err = json.Unmarshal(stdout, &internetGateways); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	for _, internetGateway := range internetGateways.InternetGateways {
		c.ArnComponents = append(c.ArnComponents, ARNComponent{
			"Internet Gateway",
			clusterName,
			internetGateway.InternetGatewayId,
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
			"-",
		})
	}

	// instances info fetch
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName), false)
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
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			componentName := "EC instance"
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "pd" {
					componentName = "PD node"
				}
				if tag["Key"] == "Component" && tag["Value"] == "tidb" {
					componentName = "TiDB node"
				}
				if tag["Key"] == "Component" && tag["Value"] == "tikv" {
					componentName = "TiKV node"
				}
				if tag["Key"] == "Component" && tag["Value"] == "dm" {
					componentName = "DM node"
				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" {
					componentName = "TiCDC node"
				}
				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					componentName = "Workstation Node"
				}
			}
			c.ArnComponents = append(c.ArnComponents, ARNComponent{
				componentName,
				clusterName,
				instance.InstanceId,
				instance.ImageId,
				instance.InstanceType,
				"-",
				instance.State.Name,
				instance.PrivateIpAddress,
				"-",
				instance.SubnetId,
			})
		}
	}
	sort.Sort(ByComponentType(c.ArnComponents))

	return nil
}

// Rollback implements the Task interface
func (c *List) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *List) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.Host)
}
