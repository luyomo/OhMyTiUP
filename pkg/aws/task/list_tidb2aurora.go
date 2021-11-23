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
	"strings"
)

type ListTiDB2Aurora struct {
	User          string
	Host          string
	ArnComponents []ARNComponent
}

// Execute implements the Task interface
func (c *ListTiDB2Aurora) Execute(ctx context.Context, clusterName, clusterType string) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.User})
	fmt.Printf("The calling functions are in the executing \n\n\n")

	// 01. VPC
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", clusterName, clusterType), false)
	if err != nil {
		return err
	}

	var vpcs Vpcs
	if err = json.Unmarshal(stdout, &vpcs); err != nil {
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

	// 02. route table
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
	if err != nil {
		return nil
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
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

	// 03. Subnets
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
	if err != nil {
		return nil
	}
	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
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

	// 04. Security Group
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-security-groups --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", clusterName), false)
	if err != nil {
		return nil
	}

	var securityGroups SecurityGroups
	if err = json.Unmarshal(stdout, &securityGroups); err != nil {
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

	// 05. VPC Peering
	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=status-code,Values=failed,expired,provisioning,active,rejected\"", clusterName), false)
	if err != nil {
		return nil
	}

	var vpcConnections VpcConnections
	if err = json.Unmarshal(stdout, &vpcConnections); err != nil {
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
			clusterInfo.pcxTidb2Aurora = pcx.VpcPeeringConnectionId
		}
	}

	// 07. Internet gateway info
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

	// 08. db subnet group
	command = fmt.Sprintf("aws rds describe-db-subnet-groups --db-subnet-group-name %s ", clusterName)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBSubnetGroupName not found") {
			// If there is no resource, go ahead
			fmt.Printf("The DB Subnet group name  has not created.\n\n\n")
		} else {
			return nil
		}
	} else {
		var dbSubnetGroups DBSubnetGroups
		if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		for _, dbSubnetGroup := range dbSubnetGroups.DBSubnetGroups {
			fmt.Printf("The object is <%#v> \n\n\n", dbSubnetGroup)

			existsResource := ExistsResource(clusterType, clusterName, dbSubnetGroup.DBSubnetGroupArn, local, ctx)
			if existsResource == true {
				c.ArnComponents = append(c.ArnComponents, ARNComponent{
					"DB Subnet",
					clusterName,
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
				})
			}

		}
	}

	// 09. db cluster parameter group
	command = fmt.Sprintf("aws rds describe-db-cluster-parameter-groups --db-cluster-parameter-group-name '%s'", clusterName)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBClusterParameterGroup not found") {
			// If there is no resource, go ahead
			fmt.Printf("The DB Cluster Parameter group has not created.\n\n\n")
		} else {
			return nil
		}
	} else {
		var dbClusterParameterGroups DBClusterParameterGroups
		if err = json.Unmarshal(stdout, &dbClusterParameterGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		for _, dbClusterParameterGroup := range dbClusterParameterGroups.DBClusterParameterGroups {
			existsResource := ExistsResource(clusterType, clusterName, dbClusterParameterGroup.DBClusterParameterGroupArn, local, ctx)
			if existsResource == true {
				c.ArnComponents = append(c.ArnComponents, ARNComponent{
					"DB Cluster Parameter Group",
					clusterName,
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
				})
			}
		}
	}
	// 10. db parameter group
	command = fmt.Sprintf("aws rds describe-db-parameter-groups --db-parameter-group-name '%s'", clusterName)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBParameterGroup not found") {
			fmt.Printf("The DB Parameter group has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		var dbParameterGroups DBParameterGroups
		if err = json.Unmarshal(stdout, &dbParameterGroups); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", dbParameterGroups)
		for _, dbParameterGroup := range dbParameterGroups.DBParameterGroups {
			existsResource := ExistsResource(clusterType, clusterName, dbParameterGroup.DBParameterGroupArn, local, ctx)
			if existsResource == true {
				c.ArnComponents = append(c.ArnComponents, ARNComponent{
					"DB Parameter Group",
					clusterName,
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
				})
			}
		}
	}

	// 11. db cluster
	command = fmt.Sprintf("aws rds describe-db-clusters --db-cluster-identifier '%s'", clusterName)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBCluster %s not found", clusterName)) {
			fmt.Printf("The DB Cluster has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return err
		}
	} else {
		var dbClusters DBClusters
		if err = json.Unmarshal(stdout, &dbClusters); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		//		fmt.Printf("The db cluster is <%#v> \n\n\n", dbClusters)
		for _, dbCluster := range dbClusters.DBClusters {
			existsResource := ExistsResource(clusterType, clusterName, dbCluster.DBClusterArn, local, ctx)
			if existsResource == true {
				c.ArnComponents = append(c.ArnComponents, ARNComponent{
					"DB Cluster",
					clusterName,
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
				})
			}
		}
	}

	// 12. db instance
	command = fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", clusterName)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", clusterName)) {
			fmt.Printf("The DB Instance has not created.\n\n\n")
		} else {
			return err
		}
	} else {
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			return err
		}
		for _, instance := range dbInstances.DBInstances {
			existsResource := ExistsResource(clusterType, clusterName, instance.DBInstanceArn, local, ctx)
			if existsResource == true {
				c.ArnComponents = append(c.ArnComponents, ARNComponent{
					"DB Instance",
					clusterName,
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
					"-",
				})
			}
		}
	}

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName), false)
	if err != nil {
		return nil
	}
	fmt.Printf("The stdout from the describe-instances is <%s> \n\n\n", string(stdout))

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
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
func (c *ListTiDB2Aurora) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTiDB2Aurora) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.Host)
}
