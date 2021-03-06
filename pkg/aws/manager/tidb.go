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
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/tisample/pkg/aws/clusterutil"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	awsutils "github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/logger"
	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
	"os"
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
type TiDBDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) TiDBDeploy(
	name string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	// 1. Preparation phase
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	exist, err := m.specManager.Exist(name)
	if err != nil {
		return err
	}

	if exist {
		// FIXME: When change to use args, the suggestion text need to be updatem.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", name).
			WithProperty(tui.SuggestionFromFormat("Please specify another cluster name"))
	}

	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	// spec.ExpandRelativeDir(topo)

	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	var (
		sshConnProps  *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
		sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	)
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = tui.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	if err := m.fillHostArch(sshConnProps, sshProxyProps, topo, &gOpt, opt.User); err != nil {
		return err
	}

	if !skipConfirm {
		if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	// if err := os.MkdirAll(m.specManager.Path(name), 0755); err != nil {
	// 	return errorx.InitializationFailed.
	// 		Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(name)).
	// 		WithProperty(tui.SuggestionFromString("Please check file system permissions and try again."))
	// }

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	globalOptions := base.GlobalOptions

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}
	clusterType := "ohmytiup-tidb"

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
		envInitTasks = append(envInitTasks, t1)
	}

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV.Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t2 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t2)
	}

	builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for tidb solution service", false, envInitTasks...)

	// if afterDeploy != nil {
	// 	afterDeploy(builder, topo)
	// }

	t := builder.Build()

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	if cntEC2Nodes > 0 {
		var t5 *task.StepDisplay
		t5 = task.NewBuilder().
			CreateTransitGateway(&sexecutor).
			CreateTransitGatewayVpcAttachment(&sexecutor, "workstation").
			CreateTransitGatewayVpcAttachment(&sexecutor, "tidb").
			CreateRouteTgw(&sexecutor, "workstation", []string{"tidb"}).
			DeployTiDB(&sexecutor, "tidb", base.AwsWSConfigs, &workstationInfo).
			DeployTiDBInstance(&sexecutor, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Prepare network resources %s:%d", globalOptions.Host, 22))

		tailctx := context.WithValue(context.Background(), "clusterName", name)
		tailctx = context.WithValue(tailctx, "clusterType", clusterType)
		builder = task.NewBuilder().
			ParallelStep("+ Deploying tidb solution service ... ...", false, t5)
		t = builder.Build()
		timer.Take("Preparation")

		if err := t.Execute(ctxt.New(tailctx, gOpt.Concurrency)); err != nil {
			if errorx.Cast(err) != nil {
				// FIXME: Map possible task errors and give suggestions.
				return err
			}
			return err
		}
	}

	timer.Take("Execution")

	// 8. Print the execution summary
	timer.Print()

	logger.OutputDebugLog("aws-nodes")
	return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyTiDBCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "ohmytiup-tidb"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		DestroyVpcPeering(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().
		ParallelStep("+ Destroying tidb solution service ... ...", false, t0)
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
		DestroyNAT(&sexecutor, "tidb").
		DestroyEC2Nodes(&sexecutor, "tidb").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	t4 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	t5 := task.NewBuilder().
		DestroyCloudFormation(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Destroying cloudformation %s ", name))

	destroyTasks = append(destroyTasks, t5)

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

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListTiDBCluster(clusterName string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb")

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	// 001. VPC listing
	tableVPC := [][]string{{"Component Name", "VPC ID", "CIDR", "Status"}}
	t1 := task.NewBuilder().ListVpc(&sexecutor, &tableVPC).BuildAsStep(fmt.Sprintf("  - Listing VPC"))
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
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// 008. NLB
	var nlb task.LoadBalancer
	t8 := task.NewBuilder().ListNLB(&sexecutor, "tidb", &nlb).BuildAsStep(fmt.Sprintf("  - Listing Load Balancer "))
	listTasks = append(listTasks, t8)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("ohmytiup-tidb"))
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

// Scale a cluster.
func (m *Manager) TiDBScale(
	name string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// exist, err := m.specManager.Exist(name)
	// if err != nil {
	// 	return err
	// }

	// if !exist {
	// 	// FIXME: When change to use args, the suggestion text need to be updatem.
	// 	return errors.New("cluster is not found")
	// }

	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	spec.ExpandRelativeDir(topo)

	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	var (
		sshConnProps  *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
		sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	)
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = tui.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	if err := m.fillHostArch(sshConnProps, sshProxyProps, topo, &gOpt, opt.User); err != nil {
		return err
	}

	if !skipConfirm {
		if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(m.specManager.Path(name), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(name)).
			WithProperty(tui.SuggestionFromString("Please check file system permissions and try again."))
	}

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	globalOptions := base.GlobalOptions

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}
	clusterType := "ohmytiup-tidb"

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV.Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t2 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t2)
	}

	builder := task.NewBuilder().ParallelStep("+ Initialize target host environments", false, envInitTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	var t5 *task.StepDisplay
	t5 = task.NewBuilder().
		ScaleTiDB(&sexecutor, "tidb", base.AwsWSConfigs, base.AwsTopoConfigs).
		BuildAsStep(fmt.Sprintf("  - Prepare Ec2  resources %s:%d", globalOptions.Host, 22))

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	builder = task.NewBuilder().
		ParallelStep("+ Initialize target host environments", false, t5)
	t = builder.Build()
	if err := t.Execute(ctxt.New(tailctx, gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	log.Infof("Cluster `%s` scaled successfully ", name)
	return nil
}

// ------------- Latency measurement
func (m *Manager) TiDBMeasureLatencyPrepareCluster(clusterName string, gOpt operator.Options) error {
	clusterType := "ohmytiup-tidb"
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb")

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	fmt.Printf(" ----- ----- Running in the prepare phase \n")
	fmt.Printf("01. Adjust the apply thread pool \n")
	fmt.Printf("02. Create database\n")
	// 02. Create the postgres objects(Database and tables)
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", "drop database if exists latencytest"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", "create database latencytest"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	fmt.Printf("03. Create ontime table and test01\n")
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "latencytest", "CREATE TABLE test01 (col01 bigint primary key auto_random, col02 int(11) NOT NULL, col03 varchar(128) DEFAULT NULL)"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_from_file %s '%s'", "latencytest", "/opt/tidb/sql/ontime_tidb.ddl"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	fmt.Printf("04. Create ontime backup table\n")
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "latencytest", "create table ontime01 like ontime;"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	fmt.Printf("05. Generate data in the ontime table\n")
	if _, _, err := (*workstation).Execute(ctx, "apt-get install -y zip", true); err != nil {
		return err
	}

	for _, file := range []string{"download_import_ontime.sh", "ontime_batch_insert.sh", "ontime_tp_insert.sh"} {
		err = (*workstation).TransferTemplate(ctx, fmt.Sprintf("templates/scripts/%s", file), fmt.Sprintf("/tmp/%s", file), "0755", []string{}, true, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("mv /tmp/%s /opt/scripts/", file), true); err != nil {
			return err
		}
	}

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/download_import_ontime.sh %s %s 2022 01 2022 01 1>/dev/null", "latencytest", "ontime01"), false, 1*time.Hour); err != nil {
		return err
	}

	return nil

}

func (m *Manager) TiDBMeasureLatencyRunCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	fmt.Printf(" ----- ----- Running in the execute phase \n")
	fmt.Printf("Parameter 01: Loop times\n")
	fmt.Printf("Parameter 02: batch size\n")
	fmt.Printf("Parameter 03: loop size\n")

	fmt.Printf("Two thread to check the data insert\n")

	clusterType := "ohmytiup-tidb"
	ctx, cancel := context.WithCancel(context.Background())
	// ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	var metricsOfLatencyWhenBatch task.MetricsOfLatencyWhenBatch
	t1 := task.NewBuilder().RunOntimeBatchInsert(&sexecutor, &metricsOfLatencyWhenBatch, &opt, &gOpt, &cancel).BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
	envInitTasks = append(envInitTasks, t1)

	t2 := task.NewBuilder().RunOntimeTpInsert(&sexecutor, &metricsOfLatencyWhenBatch, &opt, &gOpt).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
	envInitTasks = append(envInitTasks, t2)

	builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for tidb solution service", false, envInitTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 2)); err != nil {
		if errorx.Cast(err) != nil {
			return err
		}
		return err
	}

	var tableResult [][]string
	tableResult = append(tableResult, []string{"Batch Execution Time(mill)", "Batch Size", "Batch Loop", "Batch rows", "Transaction Rows", "Transaction Execution Time(Mill)", "Transaction Average execution(Mill)"})
	tableResult = append(tableResult, []string{strconv.FormatInt(metricsOfLatencyWhenBatch.BatchExecutionTime, 10),
		strconv.Itoa(metricsOfLatencyWhenBatch.BatchSize),
		strconv.Itoa(metricsOfLatencyWhenBatch.Loop),
		strconv.FormatInt(metricsOfLatencyWhenBatch.BatchTotalRows, 10),
		strconv.FormatInt(metricsOfLatencyWhenBatch.TransRow, 10),
		strconv.FormatInt(metricsOfLatencyWhenBatch.TotalExecutionTime, 10),
		strconv.FormatInt(metricsOfLatencyWhenBatch.AverageExecutionTime, 10)})

	tui.PrintTable(tableResult, true)

	return nil

}

func (m *Manager) TiDBMeasureLatencyCleanupCluster(clusterName string, gOpt operator.Options) error {
	fmt.Printf("Running in the clean phase ")
	fmt.Printf("Remove the database")
	return nil

}
