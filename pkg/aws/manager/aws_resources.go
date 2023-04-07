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

package manager

import (
	"context"
	// "errors"
	"fmt"
	// "os"
	// "strings"
	// "time"
	"github.com/fatih/color"
	// "github.com/joomcode/errorx"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	// operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	// awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	// "github.com/luyomo/OhMyTiUP/pkg/logger"
	// "github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "github.com/luyomo/OhMyTiUP/pkg/meta"
	// "github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	// perrs "github.com/pingcap/errors"
)

// Deploy a cluster.
func (m *Manager) ListAwsResources(name string) error {

	fmt.Printf("Listing aws Resources \n")
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	cyan := color.New(color.FgCyan, color.Bold)

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	tableEC2 := [][]string{{"Component Type", "Key name ", "Lanch Time", "State"}}
	t1 := task.NewBuilder().ListAwsEC2(&sexecutor, &tableEC2).BuildAsStep(fmt.Sprintf("  - Listing Resources"))
	listTasks = append(listTasks, t1)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(context.Background(), 10)); err != nil {
		return err
	}

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("EC2"))
	tui.PrintTable(tableEC2, true)

	return nil

	// listTasks = append(listTasks, t1)

	// // 002. subnets
	// tableSubnets := [][]string{{"Component Name", "Zone", "Subnet ID", "CIDR", "State", "VPC ID"}}
	// t2 := task.NewBuilder().ListNetwork(&sexecutor, &tableSubnets).BuildAsStep(fmt.Sprintf("  - Listing Subnets"))
	// listTasks = append(listTasks, t2)

	// // 003. subnets
	// tableRouteTables := [][]string{{"Component Name", "Route Table ID", "DestinationCidrBlock", "TransitGatewayId", "GatewayId", "State", "Origin"}}
	// t3 := task.NewBuilder().ListRouteTable(&sexecutor, &tableRouteTables).BuildAsStep(fmt.Sprintf("  - Listing Route Tables"))
	// listTasks = append(listTasks, t3)

	// // 004. Security Groups
	// tableSecurityGroups := [][]string{{"Component Name", "Ip Protocol", "Source Ip Range", "From Port", "To Port"}}
	// t4 := task.NewBuilder().ListSecurityGroup(&sexecutor, &tableSecurityGroups).BuildAsStep(fmt.Sprintf("  - Listing Security Groups"))
	// listTasks = append(listTasks, t4)

	// // 005. Transit gateway
	// var transitGateway task.TransitGateway
	// t5 := task.NewBuilder().ListTransitGateway(&sexecutor, &transitGateway).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway "))
	// listTasks = append(listTasks, t5)

	// // 006. Transit gateway vpc attachment
	// tableTransitGatewayVpcAttachments := [][]string{{"Component Name", "VPC ID", "State"}}
	// t6 := task.NewBuilder().ListTransitGatewayVpcAttachment(&sexecutor, &tableTransitGatewayVpcAttachments).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway vpc attachment"))
	// listTasks = append(listTasks, t6)

	// // 007. EC2
	// tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Private IP", "Public IP", "Image ID"}}
	// t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	// listTasks = append(listTasks, t7)

	// // 008. NLB
	// var nlb task.LoadBalancer
	// t8 := task.NewBuilder().ListNLB(&sexecutor, "tidb", &nlb).BuildAsStep(fmt.Sprintf("  - Listing Load Balancer "))
	// listTasks = append(listTasks, t8)

	// // *********************************************************************
	// builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	// t := builder.Build()

	// if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
	// 	return err
	// }

	// titleFont := color.New(color.FgRed, color.Bold)
	// fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("ohmytiup-kafka"))
	// fmt.Printf("Cluster Name :      %s\n\n", titleFont.Sprint(clusterName))

	// cyan := color.New(color.FgCyan, color.Bold)
	// fmt.Printf("Resource Type:      %s\n", cyan.Sprint("VPC"))
	// tui.PrintTable(tableVPC, true)

	// fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Subnet"))
	// tui.PrintTable(tableSubnets, true)

	// fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Route Table"))
	// tui.PrintTable(tableRouteTables, true)

	// fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Security Group"))
	// tui.PrintTable(tableSecurityGroups, true)

	// fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Transit Gateway"))
	// fmt.Printf("Resource ID  :      %s    State: %s \n", cyan.Sprint(transitGateway.TransitGatewayId), cyan.Sprint(transitGateway.State))
	// tui.PrintTable(tableTransitGatewayVpcAttachments, true)

	// fmt.Printf("\nLoad Balancer:      %s", cyan.Sprint(nlb.DNSName))
	// fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("EC2"))
	// tui.PrintTable(tableECs, true)

	// return nil
}
