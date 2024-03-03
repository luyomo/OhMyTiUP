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
	// "os"
	"strings"

	"github.com/joomcode/errorx"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"

	"github.com/fatih/color"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	perrs "github.com/pingcap/errors"
)

// DeployOptions contains the options for scale out.
type PDNSDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) PDNSDeploy(
	name, clusterType string,
	topoFile string,
	opt PDNSDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
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

	if !skipConfirm {
		if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	var workstationInfo, clusterInfo task.ClusterInfo

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

	t2 := task.NewBuilder().CreateTiDBCluster(&m.localExe, "tidb", base.AwsTopoConfigs, &clusterInfo).BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	task001 = append(task001, t2)

	paraTask001 := task.NewBuilder().
		CreateTransitGateway(&m.localExe).
		ParallelStep("+ Deploying all the sub components", false, task001...).
		CreateRouteTgw(&m.localExe, "workstation", []string{"tidb"}).
		RunCommonWS(&m.wsExe, &[]string{"git"}).
		DeployTiDB("tidb", base.AwsWSConfigs, base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
		DeployTiDBInstance(base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
		DeployPDNS(&m.localExe, "tidb", base.AwsWSConfigs).
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

	// spec.ExpandRelativeDir(topo)

	// base := topo.BaseTopo()
	// if sshType := gOpt.SSHType; sshType != "" {
	// 	base.GlobalOptions.SSHType = sshType
	// }

	// var (
	// 	sshConnProps  *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	// 	sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	// )
	// clusterType := "ohmytiup-pdns"
	// ctx := context.WithValue(context.Background(), "clusterName", name)
	// ctx = context.WithValue(ctx, "clusterType", clusterType)
	// if gOpt.SSHType != executor.SSHTypeNone {
	// 	var err error
	// 	if sshConnProps, err = tui.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
	// 		return err
	// 	}
	// 	if len(gOpt.SSHProxyHost) != 0 {
	// 		if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	// if err := m.fillHostArch(sshConnProps, sshProxyProps, topo, &gOpt, opt.User); err != nil {
	// 	return err
	// }

	// if !skipConfirm {
	// 	if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
	// 		return err
	// 	}
	// }

	// if err := os.MkdirAll(m.specManager.Path(name), 0755); err != nil {
	// 	return errorx.InitializationFailed.
	// 		Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(name)).
	// 		WithProperty(tui.SuggestionFromString("Please check file system permissions and try again."))
	// }

	// var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	// globalOptions := base.GlobalOptions

	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	// var workstationInfo, clusterInfo task.ClusterInfo

	// if base.AwsWSConfigs.InstanceType != "" {
	// 	fpMakeWSContext := func() error {
	// 		if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
	// 			return err
	// 		}
	// 		return nil
	// 	}
	// 	t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext).
	// 		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

	// 	envInitTasks = append(envInitTasks, t1)
	// }

	// // Todo: Need to check the tikv structure
	// cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV[0].Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	// if cntEC2Nodes > 0 {
	// 	t2 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
	// 		BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	// 	envInitTasks = append(envInitTasks, t2)
	// }

	// builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for tidb2ms solution service", false, envInitTasks...)

	// if afterDeploy != nil {
	// 	afterDeploy(builder, topo)
	// }

	// t := builder.Build()

	// if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	// var t5 *task.StepDisplay

	// t5 = task.NewBuilder().
	// 	CreateTransitGateway(&sexecutor).
	// 	CreateTransitGatewayVpcAttachment(&sexecutor, "workstation", "public").
	// 	CreateTransitGatewayVpcAttachment(&sexecutor, "tidb", "private").
	// 	CreateRouteTgw(&sexecutor, "workstation", []string{"tidb"}).
	// 	DeployTiDB("tidb", base.AwsWSConfigs, base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
	// 	DeployTiDBInstance(base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
	// 	CreateTiDBNLB(&sexecutor, "tidb", &clusterInfo).
	// 	DeployPDNS(&sexecutor, "tidb", base.AwsWSConfigs).
	// 	DeployWS(&sexecutor, "tidb", base.AwsWSConfigs).
	// 	BuildAsStep(fmt.Sprintf("  - Deploying PDNS service %s:%d", globalOptions.Host, 22))

	// tailctx := context.WithValue(context.Background(), "clusterName", name)
	// tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	// builder = task.NewBuilder().
	// 	ParallelStep("+ Deploying tidb2ms solution service ... ...", false, t5)
	// t = builder.Build()
	// if err := t.Execute(ctxt.New(tailctx, gOpt.Concurrency)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	// logger.OutputDebugLog("aws-nodes")
	// return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListPDNSService(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	if strings.ToUpper(clusterName) == "ALL" {
		tableCluster := [][]string{{"Cluster Name"}}
		ret, err := task.SearchVPCName(&sexecutor, ctx, "ohmytiup-pdns")
		if err != nil {
			return err
		}
		for _, rec := range *ret {
			tableCluster = append(tableCluster, []string{rec["Name"], "", ""})
		}

		tui.PrintTable(tableCluster, true)
		return nil
	}

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

	// 005. Transit gateway
	var transitGateway task.TransitGateway
	t5 := task.NewBuilder().ListTransitGateway(&sexecutor, &transitGateway).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway "))
	listTasks = append(listTasks, t5)

	// 006. Transit gateway vpc attachment
	tableTransitGatewayVpcAttachments := [][]string{{"Component Name", "VPC ID", "State"}}
	t6 := task.NewBuilder().ListTransitGatewayVpcAttachment(&sexecutor, &tableTransitGatewayVpcAttachments).BuildAsStep(fmt.Sprintf("  - Listing Transit gateway vpc attachment"))
	listTasks = append(listTasks, t6)

	// 007. EC2
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Private IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// 008. NLB
	var nlb elbtypes.LoadBalancer
	t8 := task.NewBuilder().ListNLB(&sexecutor, "tidb", &nlb).BuildAsStep(fmt.Sprintf("  - Listing Load Balancer "))
	listTasks = append(listTasks, t8)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("ohmytiup-tidb2ms"))
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

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyPDNSService(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().
		ParallelStep("+ Destroying tidb2ms solution service ... ...", false, t0)
	t := builder.Build()
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	var destroyTasks []*task.StepDisplay

	t1 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "tidb").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	t4 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	builder = task.NewBuilder().
		ParallelStep("+ Destroying all the componets", false, destroyTasks...)

	t = builder.Build()

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(tailctx, 5)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}

// // DestroyCluster destroy the cluster.
// func (m *Manager) DestroyPDNSService(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
// 	_, err := m.meta(name)
// 	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
// 		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
// 		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
// 		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
// 		return err
// 	}

// 	clusterType := "ohmytiup-pdns"

// 	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
// 	if err != nil {
// 		return err
// 	}

// 	t0 := task.NewBuilder().
// 		DestroyTransitGateways(&sexecutor).
// 		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

// 	builder := task.NewBuilder().
// 		ParallelStep("+ Destroying tidb2ms solution service ... ...", false, t0)
// 	t := builder.Build()
// 	ctx := context.WithValue(context.Background(), "clusterName", name)
// 	ctx = context.WithValue(ctx, "clusterType", clusterType)
// 	if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
// 		if errorx.Cast(err) != nil {
// 			// FIXME: Map possible task errors and give suggestions.
// 			return err
// 		}
// 		return err
// 	}

// 	var destroyTasks []*task.StepDisplay

// 	t1 := task.NewBuilder().
// 		DestroyEC2Nodes(&sexecutor, "tidb").
// 		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

// 	destroyTasks = append(destroyTasks, t1)

// 	t4 := task.NewBuilder().
// 		DestroyEC2Nodes(&sexecutor, "workstation").
// 		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

// 	destroyTasks = append(destroyTasks, t4)

// 	builder = task.NewBuilder().
// 		ParallelStep("+ Destroying all the componets", false, destroyTasks...)

// 	t = builder.Build()

// 	tailctx := context.WithValue(context.Background(), "clusterName", name)
// 	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
// 	if err := t.Execute(ctxt.New(tailctx, 5)); err != nil {
// 		if errorx.Cast(err) != nil {
// 			// FIXME: Map possible task errors and give suggestions.
// 			return err
// 		}
// 		return err
// 	}

// 	return nil
// }
