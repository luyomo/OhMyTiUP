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
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	perrs "github.com/pingcap/errors"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// Deploy a cluster.
func (m *Manager) TiDB2S3DeltaLakeDeploy(
	name, clusterType string,
	topoFile string,
	opt TiDB2Kafka2PgDeployOptions,
	gOpt operator.Options,
) error {
	// 1. Preparation phase
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// Get the topo file and parse it
	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	// Setup the ssh type
	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	if err := m.confirmTiDBTopology(name, topo); err != nil {
		return err
	}

	// globalOptions := base.GlobalOptions

	// -- workstation cluster   | --> routes --> | --> TiDB instance deployment
	// -- tidb cluster          |                | --> kafka insance deployment

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := m.makeExeContext(ctx, nil, &gOpt, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}

	var workstationInfo, clusterInfo, _, _ task.ClusterInfo

	var mainTask []*task.StepDisplay // tasks which are used to initialize environment

	// Parallel task to create workstation, tidb, kafka resources and transit gateway.
	var task001 []*task.StepDisplay // tasks which are used to initialize environment

    fpMakeWSContext := func() error {
        if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
            return err
        }
        return nil
    }
	t1 := task.NewBuilder().CreateWorkstationCluster(&m.localExe, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext ).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	task001 = append(task001, t1)

	t3 := task.NewBuilder().CreateTiDBCluster(&m.localExe, "tidb", base.AwsTopoConfigs, &clusterInfo).BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	task001 = append(task001, t3)

	t4 := task.NewBuilder().CreateTransitGateway(&m.localExe).BuildAsStep(fmt.Sprintf("  - Preparing the transit gateway"))
	task001 = append(task001, t4)

	// Parallel task to create tidb and kafka instances
	var task002 []*task.StepDisplay // tasks which are used to initialize environment

	t23 := task.NewBuilder().
		DeployTiDB(&m.localExe, "tidb", base.AwsWSConfigs, &workstationInfo, &m.workstation).
		DeployTiDBInstance(&m.localExe, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo, &m.workstation).
		BuildAsStep(fmt.Sprintf("  - Deploying tidb instance ... "))
	task002 = append(task002, t23)

	// The es might be lag behind the tidb/kafka cluster
	// Cluster generation -> transit gateway setup -> instance deployment
	paraTask001 := task.NewBuilder().ParallelStep("+ Deploying all the sub components for kafka solution service", false, task001...).
		CreateRouteTgw(&m.localExe, "workstation", []string{"tidb"}).
		ParallelStep("+ Deploying all the sub components for kafka solution service", false, task002...).BuildAsStep("Parallel Main step")

	// Combine the ES deployment and other resources
	mainTask = append(mainTask, paraTask001)
	mainBuilder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for kafka solution service", false, mainTask...).Build()

	// if err := paraTask001.Execute(ctxt.New(ctx, 10)); err != nil {
	if err := mainBuilder.Execute(ctxt.New(ctx, 10)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}
	timer.Take("Execution")

	// 8. Print the execution summary
	timer.Print()

	return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyS3DeltaLakeCluster(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {

	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	// gOpt.SSHUser, gOpt.IdentityFile
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}

	var destroyTasks []*task.StepDisplay

	t1 := task.NewBuilder().DestroyTransitGateways(&m.localExe).BuildAsStep("  - Removing transit gateway")
	destroyTasks = append(destroyTasks, t1)

	t4 := task.NewBuilder().DestroyNAT(&m.localExe, "tidb").DestroyEC2Nodes(&m.localExe, "tidb").BuildAsStep(fmt.Sprintf("  - Destroying  tidb cluster %s ", name))
	destroyTasks = append(destroyTasks, t4)

	builder := task.NewBuilder().ParallelStep("+ Destroying all the componets", false, destroyTasks...)

	t := builder.Build()

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(tailctx, 5)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	t10 := task.NewBuilder().DestroyEC2Nodes(&m.localExe, "workstation").BuildAsStep(fmt.Sprintf("  - Removing workstation"))

	t10.Execute(ctxt.New(tailctx, 1))

	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListS3DeltaLakeCluster(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, nil, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}
	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	// 001. VPC listing
	tableVPC := [][]string{{"Component Name", "VPC ID", "CIDR", "Status"}}
	t1 := task.NewBuilder().ListVPC(&m.localExe, &tableVPC).BuildAsStep(fmt.Sprintf("  - Listing VPC"))
	listTasks = append(listTasks, t1)

	// 002. subnets
	tableSubnets := [][]string{{"Component Name", "Zone", "Subnet ID", "CIDR", "State", "VPC ID"}}
	t2 := task.NewBuilder().ListNetwork(&m.localExe, &tableSubnets).BuildAsStep(fmt.Sprintf("  - Listing Subnets"))
	listTasks = append(listTasks, t2)

	// 003. subnets
	tableRouteTables := [][]string{{"Component Name", "Route Table ID", "DestinationCidrBlock", "TransitGatewayId", "GatewayId", "State", "Origin"}}
	t3 := task.NewBuilder().ListRouteTable(&m.localExe, &tableRouteTables).BuildAsStep(fmt.Sprintf("  - Listing Route Tables"))
	listTasks = append(listTasks, t3)

	// 004. Security Groups
	tableSecurityGroups := [][]string{{"Component Name", "Ip Protocol", "Source Ip Range", "From Port", "To Port"}}
	t4 := task.NewBuilder().ListSecurityGroup(&m.localExe, &tableSecurityGroups).BuildAsStep(fmt.Sprintf("  - Listing Security Groups"))
	listTasks = append(listTasks, t4)

	// 005. Transit gateway
	var transitGateway task.TransitGateway
	t5 := task.NewBuilder().ListTransitGateway(&m.localExe, &transitGateway).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway "))
	listTasks = append(listTasks, t5)

	// 006. Transit gateway vpc attachment
	tableTransitGatewayVpcAttachments := [][]string{{"Component Name", "VPC ID", "State"}}
	t6 := task.NewBuilder().ListTransitGatewayVpcAttachment(&m.localExe, &tableTransitGatewayVpcAttachments).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway vpc attachment"))
	listTasks = append(listTasks, t6)

	// 007. EC2
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Private IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// 008. NLB
	var nlb elbtypes.LoadBalancer
	t8 := task.NewBuilder().ListNLB(&m.localExe, "tidb", &nlb).BuildAsStep(fmt.Sprintf("  - Listing Load Balancer "))
	listTasks = append(listTasks, t8)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("ohmytiup-kafka"))
	fmt.Printf("Cluster Name :      %s\n\n", titleFont.Sprint(clusterName))

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Resource Type:      %s\n", cyan.Sprint("VPC"))
	tui.PrintTable(tableVPC, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Subnet"))
	tui.PrintTable(tableSubnets, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Route Table"))
	tui.PrintTable(tableRouteTables, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Security Group"))
	tui.PrintTable(tableSecurityGroups, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Transit Gateway"))
	fmt.Printf("Resource ID  :      %s    State: %s \n", cyan.Sprint(transitGateway.TransitGatewayId), cyan.Sprint(transitGateway.State))
	tui.PrintTable(tableTransitGatewayVpcAttachments, true)

	fmt.Printf("\nLoad Balancer:      %s", cyan.Sprint(nlb.DNSName))
	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("EC2"))
	tui.PrintTable(tableECs, true)

	return nil
}
