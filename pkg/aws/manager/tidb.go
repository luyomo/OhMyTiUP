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
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// DeployOptions contains the options for scale out.
type TiDBDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) TiDBDeploy(
	name, clusterType string,
	topoFile string,
	opt DeployOptions,
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
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyTiDBCluster(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&m.localExe).
		DestroyVpcPeering(&m.localExe, []string{"workstation"}).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().ParallelStep("+ Destroying tidb solution service ... ...", false, t0)
	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	var destroyTasks []*task.StepDisplay

	// TiDB Nodes
	t1 := task.NewBuilder().
		DestroyNAT(&m.localExe, "tidb").
		DestroyEC2Nodes(&m.localExe, "tidb").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))
	destroyTasks = append(destroyTasks, t1)

	// workstation
	t4 := task.NewBuilder().
		DestroyEC2Nodes(&m.localExe, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))
	destroyTasks = append(destroyTasks, t4)

	// Cloudformation
	t5 := task.NewBuilder().
		DestroyCloudFormation(&m.localExe).
		BuildAsStep(fmt.Sprintf("  - Destroying cloudformation %s ", name))
	destroyTasks = append(destroyTasks, t5)

	builder = task.NewBuilder().ParallelStep("+ Destroying all the componets", false, destroyTasks...)
	t = builder.Build()
	if err := t.Execute(ctxt.New(ctx, 5)); err != nil {
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
func (m *Manager) ListTiDBCluster(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
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
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint(clusterType))
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

	if nlb.DNSName != nil {
		fmt.Printf("\nLoad Balancer:      %s", cyan.Sprint(*nlb.DNSName))
	} else {
		fmt.Printf("\nLoad Balancer:      %s", cyan.Sprint(""))
	}
	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("EC2"))
	tui.PrintTable(tableECs, true)

	return nil
}

// Scale a cluster.
func (m *Manager) TiDBScale(
	name, clusterType string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

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
	// clusterType := "ohmytiup-tidb"
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	ctx = context.WithValue(ctx, "tagOwner", gOpt.TagOwner)
	ctx = context.WithValue(ctx, "tagProject", gOpt.TagProject)

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		fpMakeWSContext := func() error {
			if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
				return err
			}
			return nil
		}
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV[0].Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
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
func (m *Manager) TiDBMeasureLatencyPrepareCluster(clusterName, clusterType string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	// 01. Setup the execution environment
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.INC_AWS_ENV); err != nil {
		return err
	}

	if err := m.workstation.InstallPackages(&[]string{"zip", "sysbench"}); err != nil {
		return err
	}

	// 03. Create the necessary tidb resources
	var queries []string
	if opt.TiKVMode == "partition" {
		queries = []string{"drop database if exists latencytest", // Drop the latencytest if not exists(For batch)
			fmt.Sprintf("drop database if exists %s", opt.SysbenchDBName),             // Drop the sbtest if not exists(fosysbench)
			"DROP PLACEMENT POLICY if exists policy_online",                           // Drop the placement rule
			"DROP PLACEMENT POLICY if exists policy_batch",                            // Drop the placement rule
			"CREATE PLACEMENT POLICY policy_online CONSTRAINTS=\"[+db_type=online]\"", // Add the placement policy for online label
			"CREATE PLACEMENT POLICY policy_batch CONSTRAINTS=\"[+db_type=batch]\"",   // Add the placement policy for batch
			"set global tidb_enable_alter_placement=true",
			// Todo set one parameter to allow the database creation with placement rule
			fmt.Sprintf("create database %s PLACEMENT POLICY=policy_online", opt.SysbenchDBName), // Create the database assigned with online label
			"create database latencytest PLACEMENT POLICY=policy_batch",                          // Create the database assigned with batch label
		}
	} else {
		queries = []string{"drop database if exists latencytest",
			"create database latencytest",
			fmt.Sprintf("drop database if exists %s", opt.SysbenchDBName),
			fmt.Sprintf("create database %s", opt.SysbenchDBName),
		}
	}

	queries = append(queries, "create user if not exists `batchusr`@`%` identified by \"1234Abcd\"",
		"create user if not exists `onlineusr`@`%` identified by \"1234Abcd\"",
		"grant all on *.* to `onlineusr`@`%` ",
		"grant all on *.* to `batchusr`@`%` ",
	)

	if opt.IsolationMode == "ResourceControl" {
		queries = append(queries, "create resource group if not exists sg_online ru_per_sec=15000 priority = high burstable",
			"create resource group if not exists sg_batch ru_per_sec=2000 priority = low",
			"alter user `onlineusr`@`%` resource group sg_online",
			"alter user `batchusr`@`%` resource group sg_batch",
		)
	}

	for _, query := range queries {
		if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", query), false, 1*time.Hour); err != nil {
			return err
		}
	}

	// 04. Create ontime table to populate test data
	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_from_file %s '%s'", "latencytest", "/opt/tidb/sql/ontime_tidb.ddl"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "latencytest", "create table ontime01 like ontime;"), false, 1*time.Hour); err != nil {
		return err
	}

	//  select DB_NAME, TABLE_NAME, STORE_ID, count(*) as cnt from TIKV_REGION_PEERS t1 inner join TIKV_REGION_STATUS t2 on t1.REGION_ID = t2.REGION_ID and t2.db_name in ('sbtest', 'latencytest') group by DB_NAME, TABLE_NAME, STORE_ID order by DB_NAME, TABLE_NAME, STORE_ID;

	// 05. Data preparation from external
	for _, file := range []string{"download_import_ontime.sh", "ontime_batch_insert.sh", "ontime_shard_batch_insert.sh"} {
		if err := task.TransferToWorkstation(&m.wsExe, fmt.Sprintf("templates/scripts/%s", file), fmt.Sprintf("/opt/scripts/%s", file), "0755", []string{}); err != nil {
			return err
		}
	}

	// Download the data for ontime data population
	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/download_import_ontime.sh %s %s 2022 01 2022 01 1>/dev/null", "latencytest", "ontime01"), false, 1*time.Hour); err != nil {
		return err
	}

	// 06. Get DB info
	dbConnInfo, err := m.workstation.GetTiDBDBInfo()
	if err != nil {
		return err
	}

	tplSysbenchParam := make(map[string]string)
	tplSysbenchParam["TiDBHost"] = (*dbConnInfo).DBHost
	tplSysbenchParam["TiDBPort"] = strconv.FormatInt(int64((*dbConnInfo).DBPort), 10) // fmt.Sprintf("%s", (*dbConnInfo).DBPort)
	// tplSysbenchParam["TiDBUser"] = (*dbConnInfo).DBUser
	// tplSysbenchParam["TiDBPassword"] = (*dbConnInfo).DBPassword
	tplSysbenchParam["TiDBUser"] = "onlineusr"
	tplSysbenchParam["TiDBPassword"] = "1234Abcd"
	tplSysbenchParam["TiDBDBName"] = opt.SysbenchDBName
	tplSysbenchParam["ExecutionTime"] = strconv.FormatInt(int64(opt.SysbenchExecutionTime), 10)
	tplSysbenchParam["Thread"] = strconv.Itoa(opt.SysbenchThread)
	tplSysbenchParam["ReportInterval"] = strconv.Itoa(opt.SysbenchReportInterval)

	// 05. Setup the sysbench
	if err = task.TransferToWorkstation(&m.wsExe, "templates/config/sysbench.toml.tpl", "/opt/sysbench.toml", "0644", tplSysbenchParam); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("sysbench --config-file=%s %s --tables=%d --table-size=%d prepare", "/opt/sysbench.toml", opt.SysbenchPluginName, opt.SysbenchNumTables, opt.SysbenchNumRows), false, 1*time.Hour); err != nil {
		return err
	}

	for _, file := range []string{"tidb_common.lua", "tidb_oltp_insert.lua", "tidb_oltp_point_select.lua", "tidb_oltp_read_write.lua", "tidb_oltp_insert_simple.lua", "tidb_oltp_point_select_simple.lua", "tidb_oltp_read_write_simple.lua"} {
		if err = task.TransferToWorkstation(&m.wsExe, fmt.Sprintf("templates/scripts/sysbench/%s", file), fmt.Sprintf("/usr/share/sysbench/%s", file), "0644", []string{}); err != nil {
			return err
		}
	}

	return nil

}

func (m *Manager) TiDBMeasureLatencyRunCluster(clusterName, clusterType string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	ctx, cancel := context.WithCancel(context.Background())

	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV)
	if err != nil {
		return err
	}

	var sysbenchResult [][]string

	for idx := 0; idx < opt.RunCount; idx++ {
		if opt.IsolationMode == "ResourceControl" {
			// 01. Sinlge sysbench
			_data, err := m.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
			if err != nil {
				return err
			}
			originalCnt := (*_data)[0]["ontime_cnt"].(float64)
			fmt.Printf("The rows 001: %f \n\n\n", originalCnt)

			ctx01, cancel01 := context.WithCancel(context.Background())
			ctx01 = context.WithValue(ctx01, "clusterName", clusterName)
			ctx01 = context.WithValue(ctx01, "clusterType", clusterType)

			var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

			t1 := task.NewBuilder().RunSysbench(&m.wsExe, "/opt/sysbench.toml", &sysbenchResult, &opt, &cancel01).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
			envInitTasks = append(envInitTasks, t1)

			t := task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running %d round %s th test", idx+1, "sysbench"), false, envInitTasks...).Build()

			if err := t.Execute(ctxt.New(ctx01, 2)); err != nil {
				if errorx.Cast(err) != nil {
					return err
				}
				return err
			}

			_data, err = m.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
			if err != nil {
				return err
			}
			cnt := (*_data)[0]["ontime_cnt"].(float64) - originalCnt
			originalCnt = (*_data)[0]["ontime_cnt"].(float64)
			fmt.Printf("The rows 002: %f \n\n\n", originalCnt)
			lastItem := sysbenchResult[len(sysbenchResult)-1]
			lastItem = append([]string{fmt.Sprintf("%d", int(math.Round(cnt)))}, lastItem...)
			sysbenchResult = append(sysbenchResult[:len(sysbenchResult)-1], lastItem)

			time.Sleep(30 * time.Second)

			// 02. sysbench + insert
			if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "mysql", "alter resource group sg_batch ru_per_sec=2000 priority=high burstable"), false, 1*time.Hour); err != nil {
				return err
			}

			ctx02, cancel02 := context.WithCancel(context.Background())
			ctx02 = context.WithValue(ctx02, "clusterName", clusterName)
			ctx02 = context.WithValue(ctx02, "clusterType", clusterType)

			envInitTasks = []*task.StepDisplay{}

			t1 = task.NewBuilder().RunSysbench(&m.wsExe, "/opt/sysbench.toml", &sysbenchResult, &opt, &cancel02).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
			envInitTasks = append(envInitTasks, t1)

			t2 := task.NewBuilder().RunOntimeBatchInsert(&m.wsExe, &opt, &gOpt, "batch").BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
			envInitTasks = append(envInitTasks, t2)

			t = task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running %d round %s th test", idx+1, "sysbench"), false, envInitTasks...).Build()

			if err := t.Execute(ctxt.New(ctx02, 2)); err != nil {
				if errorx.Cast(err) != nil {
					return err
				}
				return err
			}

			_data, err = m.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
			if err != nil {
				return err
			}
			cnt = (*_data)[0]["ontime_cnt"].(float64) - originalCnt
			originalCnt = (*_data)[0]["ontime_cnt"].(float64)
			fmt.Printf("The rows 003: %f \n\n\n", originalCnt)
			lastItem = sysbenchResult[len(sysbenchResult)-1]
			lastItem = append([]string{fmt.Sprintf("%d", int(math.Round(cnt)))}, lastItem...)
			sysbenchResult = append(sysbenchResult[:len(sysbenchResult)-1], lastItem)

			time.Sleep(30 * time.Second)

			// 03. sysbench + batch
			if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "mysql", "alter resource group sg_batch ru_per_sec=2000 priority=high burstable"), false, 1*time.Hour); err != nil {
				return err
			}

			ctx03, cancel03 := context.WithCancel(context.Background())
			ctx03 = context.WithValue(ctx03, "clusterName", clusterName)
			ctx03 = context.WithValue(ctx03, "clusterType", clusterType)

			envInitTasks = []*task.StepDisplay{}

			t1 = task.NewBuilder().RunSysbench(&m.wsExe, "/opt/sysbench.toml", &sysbenchResult, &opt, &cancel03).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
			envInitTasks = append(envInitTasks, t1)

			t2 = task.NewBuilder().RunOntimeBatchInsert(&m.wsExe, &opt, &gOpt, "partition").BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
			envInitTasks = append(envInitTasks, t2)

			t = task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running %d round %s th test", idx+1, "sysbench"), false, envInitTasks...).Build()

			if err := t.Execute(ctxt.New(ctx03, 2)); err != nil {
				if errorx.Cast(err) != nil {
					return err
				}
				return err
			}

			_data, err = m.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
			if err != nil {
				return err
			}
			cnt = (*_data)[0]["ontime_cnt"].(float64) - originalCnt
			originalCnt = (*_data)[0]["ontime_cnt"].(float64)
			fmt.Printf("The rows 004: %f \n\n\n", originalCnt)
			lastItem = sysbenchResult[len(sysbenchResult)-1]
			lastItem = append([]string{fmt.Sprintf("%d", int(math.Round(cnt)))}, lastItem...)
			sysbenchResult = append(sysbenchResult[:len(sysbenchResult)-1], lastItem)
			time.Sleep(30 * time.Second)

			// 04. sysbench + batch(resource control)
			if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "mysql", "alter resource group sg_batch ru_per_sec=2000 priority=low"), false, 1*time.Hour); err != nil {
				return err
			}
			ctx04, cancel04 := context.WithCancel(context.Background())
			ctx04 = context.WithValue(ctx04, "clusterName", clusterName)
			ctx04 = context.WithValue(ctx04, "clusterType", clusterType)

			envInitTasks = []*task.StepDisplay{}

			t1 = task.NewBuilder().RunSysbench(&m.wsExe, "/opt/sysbench.toml", &sysbenchResult, &opt, &cancel04).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
			envInitTasks = append(envInitTasks, t1)

			t2 = task.NewBuilder().RunOntimeBatchInsert(&m.wsExe, &opt, &gOpt, "partition").BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
			envInitTasks = append(envInitTasks, t2)

			t = task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running %d round %s th test", idx+1, "sysbench"), false, envInitTasks...).Build()

			if err := t.Execute(ctxt.New(ctx04, 2)); err != nil {
				if errorx.Cast(err) != nil {
					return err
				}
				return err
			}

			_data, err = m.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
			if err != nil {
				return err
			}
			cnt = (*_data)[0]["ontime_cnt"].(float64) - originalCnt
			originalCnt = (*_data)[0]["ontime_cnt"].(float64)
			fmt.Printf("The rows 005: %f \n\n\n", originalCnt)
			lastItem = sysbenchResult[len(sysbenchResult)-1]
			lastItem = append([]string{fmt.Sprintf("%d", int(math.Round(cnt)))}, lastItem...)
			sysbenchResult = append(sysbenchResult[:len(sysbenchResult)-1], lastItem)

		} else {
			for idxBatchSize, batchSize := range strings.Split(opt.BatchSizeArray, ",") {
				// 01. Set the context
				ctx, cancel = context.WithCancel(context.Background())
				ctx = context.WithValue(ctx, "clusterName", clusterName)
				ctx = context.WithValue(ctx, "clusterType", clusterType)

				// 02. Prepare the task
				var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

				t1 := task.NewBuilder().RunSysbench(&m.wsExe, "/opt/sysbench.toml", &sysbenchResult, &opt, &cancel).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
				envInitTasks = append(envInitTasks, t1)

				// 03. If the batch size is not x, run the data batch insert
				if batchSize != "x" {
					opt.BatchSize, err = strconv.Atoi(batchSize)
					if err != nil {
						return err
					}

					t2 := task.NewBuilder().RunOntimeBatchInsert(&m.wsExe, &opt, &gOpt, "insert").BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
					envInitTasks = append(envInitTasks, t2)
				} else {
					opt.BatchSize = 0
				}

				// 04. Run sysbench
				builder := task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running %d round %d th test for batch-size: %s", idx+1, idxBatchSize+1, batchSize), false, envInitTasks...)

				t := builder.Build()

				if err := t.Execute(ctxt.New(ctx, 2)); err != nil {
					if errorx.Cast(err) != nil {
						return err
					}
					return err
				}

				time.Sleep(20 * time.Second)
			}
		}
	}

	tui.PrintTable(sysbenchResult, true)

	return nil
}

func (m *Manager) TiDBMeasureLatencyCleanupCluster(clusterName, clusterType string, gOpt operator.Options) error {
	fmt.Printf("Running in the clean phase ")
	fmt.Printf("Remove the database")
	return nil

}

// ------------- recursive query performance on TiFlash
func (m *Manager) TiDBRecursivePrepareCluster(clusterName, clusterType string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.INC_AWS_ENV); err != nil {
		return err
	}

	for _, file := range []string{"generateUsers.sh", "generatePayment.sh", "generatePayment2CSV.sh"} {
		if err := task.TransferToWorkstation(&m.wsExe, fmt.Sprintf("templates/scripts/recursive-on-tiflash/%s", file), fmt.Sprintf("/opt/scripts/%s", file), "0755", []string{}); err != nil {
			return err
		}
	}

	queries := []string{
		"drop table if exists users",
		"drop table if exists payment",
		"create table users (id bigint primary key auto_increment, name varchar(32))",
		"create table payment(id bigint primary key auto_random, payer varchar(32), receiver varchar(32), pay_amount bigint)",
	}

	for _, command := range queries {
		if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", command), false, 1*time.Hour); err != nil {
			return err
		}
	}

	dbConnInfo, err := m.workstation.GetTiDBDBInfo()
	if err != nil {
		return err
	}

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t1)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var pdIP, tidbIP string
	for _, row := range tableECs {
		if row[0] == "pd" {
			pdIP = row[5]
		}
		if row[0] == "tidb" {
			tidbIP = row[5]
		}

	}

	type TplLightningParam struct {
		TiDBHost     string
		TiDBPort     int
		TiDBUser     string
		TiDBPassword string
		TiDBDBName   string
		PDIP         string
	}

	tplLightningParam := TplLightningParam{
		TiDBHost:     tidbIP,
		TiDBPort:     (*dbConnInfo).DBPort,
		TiDBUser:     (*dbConnInfo).DBUser,
		TiDBPassword: (*dbConnInfo).DBPassword,
		TiDBDBName:   "test",
		PDIP:         pdIP,
	}

	if err = task.TransferToWorkstation(&m.wsExe, "templates/scripts/recursive-on-tiflash/tidb-lightning.toml.tpl", "/opt/tidb-lightning.toml", "0644", tplLightningParam); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("wget -P /tmp https://download.pingcap.org/tidb-community-toolkit-%s-linux-amd64.tar.gz", "v6.2.0"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("tar -xf /tmp/tidb-community-toolkit-%s-linux-amd64.tar.gz -C /tmp", "v6.2.0"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, "mkdir -p /tmp/recursive-data", false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, "mkdir -p /opt/bin", true, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("tar -xf /tmp/tidb-community-toolkit-%s-linux-amd64/tidb-lightning-v6.2.0-linux-amd64.tar.gz -C /opt/bin", "v6.2.0"), true, 1*time.Hour); err != nil {
		return err
	}

	return nil
}

func (m *Manager) TiDBRecursiveRunCluster(clusterName, clusterType string, numUsers, numPayments string, gOpt operator.Options) error {
	// clusterType := "ohmytiup-tidb"
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	varNumUsers, err := task.ParseRangeData(numUsers)
	fmt.Printf("The user is <%#v> \n", *varNumUsers)
	if err != nil {
		return err
	}

	varNumPayments, err := task.ParseRangeData(numPayments)
	fmt.Printf("The user is <%#v> \n", *varNumPayments)
	if err != nil {
		return err
	}

	for _, userNum := range *varNumUsers {
		fmt.Printf("The user number is <%d> \n", userNum)
		// Create two tables
		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "truncate table users"), false, 1*time.Hour); err != nil {
			return err
		}

		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "truncate table payment"), false, 1*time.Hour); err != nil {
			return err
		}

		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/generateUsers.sh %d", userNum), false, 1*time.Hour); err != nil {
			return err
		}

		for _, paymentNum := range *varNumPayments {
			if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/generatePayment2CSV.sh %d %d %s", userNum, paymentNum, "/tmp/recursive-data/test.payment.csv"), false, 1*time.Hour); err != nil {
				return err
			}

			if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/bin/tidb-lightning --config=/opt/tidb-lightning.toml"), false, 1*time.Hour); err != nil {
				return err
			}

			// if err = task.TransferToWorkstation(workstation, "templates/scripts/recursive-on-tiflash/recursive.sql.tpl", "/opt/recursive.sql", "0755", map[string]string{"RecursiveNum": "3"}); err != nil {
			// 	return err
			// }

			// if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_from_file %s %s", "test", "/opt/recursive.sql"), false, 1*time.Hour); err != nil {
			// 	return err
			// }

		}
	}

	return nil
}

func (m *Manager) TiDBPerfRecursiveCleanupCluster(clusterName, clusterType string, gOpt operator.Options) error {
	fmt.Printf("Running in the clean phase ")
	fmt.Printf("Remove the database")
	return nil

}

func (m *Manager) InstallThanos(
	clusterName, clusterType string,
	opt operator.ThanosS3Config,
	gOpt operator.Options,
) error {

	// clusterType := "ohmytiup-tidb"
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	// 03. prepare task
	t1 := task.NewBuilder().DeployThanos(&opt, &gOpt).BuildAsStep(fmt.Sprintf("  - Install thanos"))

	// 04. Execute task
	if err := t1.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {

		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}
