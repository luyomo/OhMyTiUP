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
	"os"
	"strconv"
	"strings"
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

	t := builder.Build()

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	ctx = context.WithValue(ctx, "tagEmail", gOpt.TagEmail)
	ctx = context.WithValue(ctx, "tagProject", gOpt.TagProject)
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
		DestroyVpcPeering(&sexecutor, []string{"workstation"}).
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
	ctx = context.WithValue(ctx, "tagEmail", gOpt.TagEmail)
	ctx = context.WithValue(ctx, "tagProject", gOpt.TagProject)

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
func (m *Manager) TiDBMeasureLatencyPrepareCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
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

	// 01. Install the required package
	if _, _, err := (*workstation).Execute(ctx, "apt-get install -y zip sysbench", true); err != nil {
		return err
	}

	// 02. Create the postgres objects(Database and tables)
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

	for _, query := range queries {
		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", query), false, 1*time.Hour); err != nil {
			return err
		}
	}

	// Create ontime table
	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_from_file %s '%s'", "latencytest", "/opt/tidb/sql/ontime_tidb.ddl"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "latencytest", "create table ontime01 like ontime;"), false, 1*time.Hour); err != nil {
		return err
	}

	//  select DB_NAME, TABLE_NAME, STORE_ID, count(*) as cnt from TIKV_REGION_PEERS t1 inner join TIKV_REGION_STATUS t2 on t1.REGION_ID = t2.REGION_ID and t2.db_name in ('sbtest', 'latencytest') group by DB_NAME, TABLE_NAME, STORE_ID order by DB_NAME, TABLE_NAME, STORE_ID;

	// - Data preparation
	for _, file := range []string{"download_import_ontime.sh", "ontime_batch_insert.sh"} {
		if err = task.TransferToWorkstation(workstation, fmt.Sprintf("templates/scripts/%s", file), fmt.Sprintf("/opt/scripts/%s", file), "0755", []string{}); err != nil {
			return err
		}
	}

	// Download the data for ontime data population
	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/download_import_ontime.sh %s %s 2022 01 2022 01 1>/dev/null", "latencytest", "ontime01"), false, 1*time.Hour); err != nil {
		return err
	}

	// Fetch the TiDB connection info
	dbConnInfo, err := task.ReadTiDBConntionInfo(workstation, "tidb-db-info.yml")
	if err != nil {
		return err
	}

	type TplSysbenchParam struct {
		TiDBHost       string
		TiDBPort       int
		TiDBUser       string
		TiDBPassword   string
		TiDBDBName     string
		ExecutionTime  int64
		Thread         int
		ReportInterval int
	}

	tplSysbenchParam := TplSysbenchParam{
		TiDBHost:       (*dbConnInfo).DBHost,
		TiDBPort:       (*dbConnInfo).DBPort,
		TiDBUser:       (*dbConnInfo).DBUser,
		TiDBPassword:   (*dbConnInfo).DBPassword,
		TiDBDBName:     opt.SysbenchDBName,
		ExecutionTime:  opt.SysbenchExecutionTime,
		Thread:         opt.SysbenchThread,
		ReportInterval: opt.SysbenchReportInterval,
	}

	if err = task.TransferToWorkstation(workstation, "templates/config/sysbench.toml.tpl", "/opt/sysbench.toml", "0644", tplSysbenchParam); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("sysbench --config-file=%s %s --tables=%d --table-size=%d prepare", "/opt/sysbench.toml", opt.SysbenchPluginName, opt.SysbenchNumTables, opt.SysbenchNumRows), false, 1*time.Hour); err != nil {
		return err
	}

	for _, file := range []string{"tidb_common.lua", "tidb_oltp_insert.lua", "tidb_oltp_point_select.lua", "tidb_oltp_read_write.lua", "tidb_oltp_insert_simple.lua", "tidb_oltp_point_select_simple.lua", "tidb_oltp_read_write_simple.lua"} {
		if err = task.TransferToWorkstation(workstation, fmt.Sprintf("templates/scripts/sysbench/%s", file), fmt.Sprintf("/usr/share/sysbench/%s", file), "0644", []string{}); err != nil {
			return err
		}
	}

	return nil

}

func (m *Manager) TiDBMeasureLatencyRunCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	clusterType := "ohmytiup-tidb"
	ctx, cancel := context.WithCancel(context.Background())

	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var sysbenchResult [][]string

	for idx := 0; idx < opt.RunCount; idx++ {
		for idxBatchSize, batchSize := range strings.Split(opt.BatchSizeArray, ",") {
			ctx, cancel = context.WithCancel(context.Background())
			ctx = context.WithValue(ctx, "clusterName", clusterName)
			ctx = context.WithValue(ctx, "clusterType", clusterType)

			var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

			t1 := task.NewBuilder().RunSysbench(&sexecutor, "/opt/sysbench.toml", &sysbenchResult, &opt, &gOpt, &cancel).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
			envInitTasks = append(envInitTasks, t1)

			if batchSize != "x" {
				opt.BatchSize, err = strconv.Atoi(batchSize)
				if err != nil {
					return err
				}

				t2 := task.NewBuilder().RunOntimeBatchInsert(&sexecutor, &opt, &gOpt).BuildAsStep(fmt.Sprintf("  - Running Ontime batch"))
				envInitTasks = append(envInitTasks, t2)
			} else {
				opt.BatchSize = 0
			}

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

	tui.PrintTable(sysbenchResult, true)

	return nil
}

func (m *Manager) TiDBMeasureLatencyCleanupCluster(clusterName string, gOpt operator.Options) error {
	fmt.Printf("Running in the clean phase ")
	fmt.Printf("Remove the database")
	return nil

}

// ------------- recursive query performance on TiFlash
func (m *Manager) TiDBRecursivePrepareCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
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

	for _, file := range []string{"generateUsers.sh", "generatePayment.sh", "generatePayment2CSV.sh"} {
		if err = task.TransferToWorkstation(workstation, fmt.Sprintf("templates/scripts/recursive-on-tiflash/%s", file), fmt.Sprintf("/opt/scripts/%s", file), "0755", []string{}); err != nil {
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
		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", command), false, 1*time.Hour); err != nil {
			return err
		}
	}

	dbConnInfo, err := task.ReadTiDBConntionInfo(workstation, "tidb-db-info.yml")
	if err != nil {
		return err
	}

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
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

	if err = task.TransferToWorkstation(workstation, "templates/scripts/recursive-on-tiflash/tidb-lightning.toml.tpl", "/opt/tidb-lightning.toml", "0644", tplLightningParam); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("wget -P /tmp https://download.pingcap.org/tidb-community-toolkit-%s-linux-amd64.tar.gz", "v6.2.0"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("tar -xf /tmp/tidb-community-toolkit-%s-linux-amd64.tar.gz -C /tmp", "v6.2.0"), false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, "mkdir -p /tmp/recursive-data", false, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, "mkdir -p /opt/bin", true, 1*time.Hour); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("tar -xf /tmp/tidb-community-toolkit-%s-linux-amd64/tidb-lightning-v6.2.0-linux-amd64.tar.gz -C /opt/bin", "v6.2.0"), true, 1*time.Hour); err != nil {
		return err
	}

	return nil
}

func (m *Manager) TiDBRecursiveRunCluster(clusterName string, numUsers, numPayments string, gOpt operator.Options) error {
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

func (m *Manager) TiDBPerfRecursiveCleanupCluster(clusterName string, gOpt operator.Options) error {
	fmt.Printf("Running in the clean phase ")
	fmt.Printf("Remove the database")
	return nil

}
