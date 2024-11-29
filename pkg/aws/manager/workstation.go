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
	// "github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"

	"github.com/luyomo/OhMyTiUP/pkg/meta"
	// "github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"

	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

// To resolve:
// 1. Takes time to prepare aws resource, a lot of manual work
// 2. Gather all the IP resources to make one tiup yaml file
// 3. Hard to know what's the structure of TiDB Cluster
// 4. Again, hard to scale out/in for the existing structure
// 5. Hard to clean all the resources

// Todo:
// 1. Add monitoring to workstation
// 2. Add port open on the security
// 3. Added the version

// DeployOptions contains the options for scale out.
// type TiDBDeployOptions struct {
// 	User              string // username to login to the SSH server
// 	IdentityFile      string // path to the private key file
// 	UsePassword       bool   // use password instead of identity file for ssh connection
// 	IgnoreConfigCheck bool   // ignore config check result
// }

// Deploy a cluster.
func (m *Manager) WorkstationDeploy(
	name, clusterType string,
	topoFile string,
	opt DeployOptions,
	// afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {

	// 01. Preparation phase
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 02. Get the topo file and parse it
	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	// 03. Setup the ssh type
	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	// 04. Confirm the topo config
	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}

	var workstationInfo task.ClusterInfo

	var task001 []*task.StepDisplay // tasks which are used to initialize environment

	fpMakeWSContext := func() error {
		if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
			return err
		}

		return nil
	}
	t1 := task.NewBuilder().
		CreateWorkstationCluster(&m.localExe, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	task001 = append(task001, t1)

	paraTask001 := task.NewBuilder().
		CreateTransitGateway(&m.localExe).
		ParallelStep("+ Deploying all the sub components", false, task001...).
		BuildAsStep("Parallel Main step")

	if err := paraTask001.Execute(ctxt.New(ctx, 10)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	timer.Take("Execution")

	// 8. Print the execution summary
	timer.Print()

	logger.OutputDebugLog("aws-nodes")
	return nil
	// --------------------------

	// // 1. Preparation phase
	// var timer awsutils.ExecutionTimer
	// timer.Initialize([]string{"Step", "Duration(s)"})

	// if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
	// 	return err
	// }

	// exist, err := m.specManager.Exist(name)
	// if err != nil {
	// 	return err
	// }

	// if exist {
	// 	// FIXME: When change to use args, the suggestion text need to be updatem.
	// 	return errDeployNameDuplicate.
	// 		New("Cluster name '%s' is duplicated", name).
	// 		WithProperty(tui.SuggestionFromFormat("Please specify another cluster name"))
	// }

	// metadata := m.specManager.NewMetadata()
	// topo := metadata.GetTopology()

	// if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
	// 	return err
	// }

	// base := topo.BaseTopo()

	// if sshType := gOpt.SSHType; sshType != "" {
	// 	base.GlobalOptions.SSHType = sshType
	// }

	// if !skipConfirm {
	// 	if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
	// 		return err
	// 	}
	// }

	// var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	// // globalOptions := base.GlobalOptions

	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }
	// clusterType := "ohmytiup-workstation"

	// ctx := context.WithValue(context.Background(), "clusterName", name)
	// ctx = context.WithValue(ctx, "clusterType", clusterType)
	// ctx = context.WithValue(ctx, "tagOwner", gOpt.TagOwner)
	// ctx = context.WithValue(ctx, "tagProject", gOpt.TagProject)

	// var workstationInfo task.ClusterInfo

	// if base.AwsWSConfigs.InstanceType == "" {
	// 	return errors.New("No workstation instance is specified")
	// }

	// fpMakeWSContext := func() error {
	// 	if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }
	// t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext).
	// 	BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	// envInitTasks = append(envInitTasks, t1)

	// builder := task.NewBuilder().ParallelStep("+ Deploying workstation ... ...", false, envInitTasks...)

	// t := builder.Build()

	// if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	// timer.Take("Execution")

	// // 8. Print the execution summary
	// timer.Print()

	// logger.OutputDebugLog("aws-nodes")
	// return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyWorkstation(name, clusterType string, gOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	// clusterType := "ohmytiup-workstation"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var destroyTasks []*task.StepDisplay

	t4 := task.NewBuilder().
		DestroyVpcPeering(&sexecutor, []string{"workstation"}).
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	builder := task.NewBuilder().
		ParallelStep("+ Destroying all the componets", false, destroyTasks...)

	t := builder.Build()

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(tailctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListWorkstation(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	// clusterType := "ohmytiup-workstation"
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var accountID string
	t0 := task.NewBuilder().ListAccount(&sexecutor, &accountID).BuildAsStep(fmt.Sprintf("  - List Account"))
	listTasks = append(listTasks, t0)

	// 001. VPC listing
	tableVPC := [][]string{{"Component Name", "VPC ID", "CIDR", "Status"}}
	t1 := task.NewBuilder().ListVPC(&sexecutor, &tableVPC).BuildAsStep(fmt.Sprintf("  - Listing VPC"))
	listTasks = append(listTasks, t1)

	// 002. subnets
	tableSubnets := [][]string{{"Component Name", "Zone", "Subnet ID", "CIDR", "State", "VPC ID"}}
	t2 := task.NewBuilder().ListNetwork(&sexecutor, &tableSubnets).BuildAsStep(fmt.Sprintf("  - Listing Subnets"))
	listTasks = append(listTasks, t2)

	// 003. subnets
	tableRouteTables := [][]string{{"Component Name", "Route Table ID", "DestinationCidrBlock", "TransitGatewayId", "GatewayId", "State", "Origin"}}
	t3 := task.NewBuilder().ListRouteTable(&sexecutor, &tableRouteTables).BuildAsStep(fmt.Sprintf("  - Listing Route Tables"))
	listTasks = append(listTasks, t3)

	// 004. Security Groups
	tableSecurityGroups := [][]string{{"Component Name", "Ip Protocol", "Source Ip Range", "From Port", "To Port"}}
	t4 := task.NewBuilder().ListSecurityGroup(&sexecutor, &tableSecurityGroups).BuildAsStep(fmt.Sprintf("  - Listing Security Groups"))
	listTasks = append(listTasks, t4)

	// 007. EC2
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Private IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Account ID   :      %s\n", titleFont.Sprint(accountID))
	fmt.Printf("Cluster Type:       %s\n", titleFont.Sprint(clusterType))
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

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("EC2"))
	tui.PrintTable(tableECs, true)

	return nil
}
