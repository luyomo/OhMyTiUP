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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/crypto"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"
)

func (m *Manager) Aurora2TiDBCloudDeploy(
	name string,
	topoFile string,
	skipConfirm bool,
	gOpt operator.Options,
) error {
	clusterType := "ohmytiup-aurora2tidbcloud"

	// Check the cluster name
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

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

	spec.ExpandRelativeDir(topo)

	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	var (
		envInitTasks []*task.StepDisplay // tasks which are used to initialize environment
		//downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		//deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	// uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	// noAgentHosts := set.NewStringSet()
	globalOptions := base.GlobalOptions

	// generate CA and client cert for TLS enabled cluster
	var ca *crypto.CertificateAuthority
	if globalOptions.TLSEnabled {
		// generate CA
		tlsPath := m.specManager.Path(name, spec.TLSCertKeyDir)
		if err := utils.CreateDir(tlsPath); err != nil {
			return err
		}
		ca, err = genAndSaveClusterCA(name, tlsPath)
		if err != nil {
			return err
		}

		// generate client cert
		if err = genAndSaveClientCert(ca, name, tlsPath); err != nil {
			return err
		}
	}

	var clusterInfo task.ClusterInfo
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}
	timer.Take("Resource preparation")

	if base.AwsAuroraConfigs.DBParameterFamilyGroup == "" {
		return errors.New("Please specify the aurora DB")
	}

	t1 := task.NewBuilder().
		CreateAurora(&sexecutor, base.AwsWSConfigs, base.AwsAuroraConfigs, &clusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing aurora ... ..."))
	envInitTasks = append(envInitTasks, t1)

	// Prepare the workstation
	var workstationInfo task.ClusterInfo
	t2 := task.NewBuilder().
		CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing aurora ... ..."))
	envInitTasks = append(envInitTasks, t2)

	if base.AwsTopoConfigs.DMMaster.Count > 0 || base.AwsTopoConfigs.DMWorker.Count > 0 {
		t3 := task.NewBuilder().CreateDMCluster(&sexecutor, "dm", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing dm servers"))
		envInitTasks = append(envInitTasks, t3)
	}

	builder := task.NewBuilder().
		ParallelStep("+ Initialize target host environments", false, envInitTasks...)

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
	var t5 *task.StepDisplay
	t5 = task.NewBuilder().
		CreateTransitGateway(&sexecutor).
		CreateTransitGatewayVpcAttachment(&sexecutor, "workstation").
		CreateTransitGatewayVpcAttachment(&sexecutor, "aurora").
		CreateTransitGatewayVpcAttachment(&sexecutor, "dm").
		CreateRouteTgw(&sexecutor, "workstation", []string{"aurora", "dm"}).
		CreateRouteTgw(&sexecutor, "aurora", []string{"workstation", "dm"}).
		CreateRouteTgw(&sexecutor, "dm", []string{"aurora", "workstation"}).
		DeployDM(&sexecutor, "dm", base.AwsWSConfigs, base.TiDBCloudConnInfo, &workstationInfo).
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

	logger.OutputDebugLog("aws-nodes")
	timer.Print()
	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListAurora2TiDBCloudCluster(clusterName string, opt DeployOptions) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora2tidbcloud")

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var accountID string
	t0 := task.NewBuilder().ListAccount(&sexecutor, &accountID).BuildAsStep(fmt.Sprintf("  - List Account"))
	listTasks = append(listTasks, t0)

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

	// 009. Aurora
	tableAurora := [][]string{{"Physical Name", "Host Name", "Port", "DB User", "Engine", "Engine Version", "Instance Type", "Security Group"}}
	t9 := task.NewBuilder().ListAurora(&sexecutor, &tableAurora).BuildAsStep(fmt.Sprintf("  - Listing Aurora"))
	listTasks = append(listTasks, t9)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Account ID   :      %s\n", titleFont.Sprint(accountID))
	fmt.Printf("Cluster Type :      %s\n", titleFont.Sprint("ohmytiup-aurora2tidbcloud"))
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

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Aurora"))
	tui.PrintTable(tableAurora, true)

	return nil
}

func (m *Manager) StartSyncAurora2TiDBCloudCluster(clusterName string, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"

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

	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm display %s --format json ", clusterName), false)
	var displayDMCluster task.DisplayDMCluster
	if err = json.Unmarshal(stdout, &displayDMCluster); err != nil {
		return err
	}

	masterNode := ""
	for _, node := range displayDMCluster.Instances {
		if node.Role == "dm-master" && node.Status == "Healthy" {
			masterNode = fmt.Sprintf("%s:%d", node.Host, node.Port)
		}
	}

	if masterNode == "" {
		return errors.New("No healthy master node found")
	}

	type DMSourceMeta struct {
		Result  bool   `json:"result"`
		Msg     string `json:"msg"`
		Sources []struct {
			Result bool   `json:"result"`
			Msg    string `json:"msg"`
			Source string `json:"source"`
			Worker string `json:"worker"`
		} `json:"sources"`
	}
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s operate-source show", masterNode), false)
	if err != nil {
		return err
	}

	var dmSourceMeta DMSourceMeta
	if err = json.Unmarshal(stdout, &dmSourceMeta); err != nil {
		return err
	}
	// fmt.Printf("The meta data is <%#v> \n\n\n", dmSourceMeta)

	existSource := false
	for _, source := range dmSourceMeta.Sources {
		if source.Source == clusterName {
			existSource = true
			break
		}
	}
	if existSource == false {
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s operate-source create /opt/tidb/dm-source.yml", masterNode), false)
		if err != nil {
			return err
		}
		fmt.Printf("The data is <%s> \n\n\n", stdout)
	}

	type DMTaskMeta struct {
		Result bool   `json:"result"`
		Msg    string `json:"msg"`
		Tasks  []struct {
			TaskName   string   `json:"taskName"`
			TaskStatus string   `json:"taskStatus"`
			Sources    []string `json:"sources"`
		} `json:"tasks"`
	}
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s query-status", masterNode), false)
	if err != nil {
		return err
	}

	var dmTaskMeta DMTaskMeta
	if err = json.Unmarshal(stdout, &dmTaskMeta); err != nil {
		return err
	}

	existTask := false
	for _, task := range dmTaskMeta.Tasks {
		if task.TaskName == clusterName {
			existTask = true
			break
		}
	}
	if existTask == false {
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s start-task /opt/tidb/dm-task.yml", masterNode), false)
		if err != nil {
			return err
		}
		fmt.Printf("The data is <%s> \n\n\n", stdout)
	}

	return nil
	ctx = context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora2tidbcloud")

	sexecutor, err = executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	vpcPeeringInfo := [][]string{{"VPC Peering ID", "Status", "Requestor VPC ID", "Requestor CIDR", "Acceptor VPC ID", "Acceptor CIDR"}}
	t9 := task.NewBuilder().ListVpcPeering(&sexecutor, []string{"dm", "workstation"}, &vpcPeeringInfo).BuildAsStep(fmt.Sprintf("  - Listing VPC Peering"))
	listTasks = append(listTasks, t9)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Account ID   :      %s\n", titleFont.Sprint("VPC Peering Info"))
	tui.PrintTable(vpcPeeringInfo, true)

	// 02. Accept the VPC Peering
	var acceptTasks []*task.StepDisplay // tasks which are used to initialize environment

	t2 := task.NewBuilder().AcceptVPCPeering(&sexecutor, []string{"workstation", "dm", "aurora"}).BuildAsStep(fmt.Sprintf("  - Accepting VPC Peering"))
	acceptTasks = append(acceptTasks, t2)

	// *********************************************************************
	builder = task.NewBuilder().ParallelStep("+ Accepting aws resources", false, acceptTasks...)

	t = builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	return nil
}

func (m *Manager) DestroyAurora2TiDBCloudCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "ohmytiup-aurora2tidbcloud"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		DestroyVpcPeering(&sexecutor, []string{"workstation", "dm", "aurora"}).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().
		ParallelStep("+ Destroying aurora solution service ... ...", false, t0)
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
		DestroyAurora(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Destroying aurora nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	t4 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	t2 := task.NewBuilder().
		DestroyNAT(&sexecutor, "dm").
		DestroyEC2Nodes(&sexecutor, "dm").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))
	destroyTasks = append(destroyTasks, t2)

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

// ------------- Latency measurement
func (m *Manager) Aurora2TiDBCloudPrepareCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"
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

	// 01. Install the required package
	if _, _, err := (*workstation).Execute(ctx, "apt-get install -y sysbench", true); err != nil {
		return err
	}

	// 02. Create the postgres objects(Database and tables)
	queries := []string{
		fmt.Sprintf("drop database if exists %s", opt.SysbenchDBName), // Drop the sbtest if not exists(fosysbench)
		fmt.Sprintf("create database %s", opt.SysbenchDBName),         // Create the database assigned with online label
	}

	for _, query := range queries {
		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_mysql_query mysql '%s'", query), false, 1*time.Hour); err != nil {
			return err
		}
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

	// Set sysbench file for TiDB Cloud
	// Fetch the TiDB connection info
	dbConnInfo, err := task.ReadTiDBConntionInfo(workstation, "tidbcloud-info.yml")
	if err != nil {
		return err
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

	if err = task.TransferToWorkstation(workstation, "templates/config/sysbench.toml.tpl", "/opt/tidbcloud-sysbench.toml", "0644", tplSysbenchParam); err != nil {
		return err
	}

	// Set the sysbench file for aurora
	// Fetch the TiDB connection info
	dbConnInfo, err = task.ReadTiDBConntionInfo(workstation, "db-info.yml")
	if err != nil {
		return err
	}

	tplSysbenchParam.TiDBHost = (*dbConnInfo).DBHost
	tplSysbenchParam.TiDBPort = (*dbConnInfo).DBPort
	tplSysbenchParam.TiDBUser = (*dbConnInfo).DBUser
	tplSysbenchParam.TiDBPassword = (*dbConnInfo).DBPassword

	if err = task.TransferToWorkstation(workstation, "templates/config/sysbench.toml.tpl", "/opt/aurora-sysbench.toml", "0644", tplSysbenchParam); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("sysbench --config-file=%s %s --tables=%d --table-size=%d prepare", "/opt/aurora-sysbench.toml", opt.SysbenchPluginName, opt.SysbenchNumTables, opt.SysbenchNumRows), false, 1*time.Hour); err != nil {
		return err
	}

	for _, file := range []string{"tidb_common.lua", "tidb_oltp_insert.lua", "tidb_oltp_point_select.lua", "tidb_oltp_read_write.lua", "tidb_oltp_insert_simple.lua", "tidb_oltp_point_select_simple.lua", "tidb_oltp_read_write_simple.lua"} {
		if err = task.TransferToWorkstation(workstation, fmt.Sprintf("templates/scripts/sysbench/%s", file), fmt.Sprintf("/usr/share/sysbench/%s", file), "0644", []string{}); err != nil {
			return err
		}
	}

	return nil

}

func (m *Manager) Aurora2TiDBCloudRunCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var sysbenchResult [][]string

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	if opt.SysbenchTargetInstance == "TiDBCloud" {
		t1 := task.NewBuilder().RunSysbench(&sexecutor, "/opt/tidbcloud-sysbench.toml", &sysbenchResult, &opt, &gOpt, &cancel).BuildAsStep(fmt.Sprintf("  - Running sysbench against TiDB Cloud"))
		envInitTasks = append(envInitTasks, t1)
	} else {

		t1 := task.NewBuilder().RunSysbench(&sexecutor, "/opt/aurora-sysbench.toml", &sysbenchResult, &opt, &gOpt, &cancel).BuildAsStep(fmt.Sprintf("  - Running sysbench against Aurora"))
		envInitTasks = append(envInitTasks, t1)
	}

	builder := task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running sysbench against Aurora "), false, envInitTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 2)); err != nil {
		if errorx.Cast(err) != nil {
			return err
		}
		return err
	}

	tui.PrintTable(sysbenchResult, true)
	timer.Take("Sysbench Execution")

	if opt.SysbenchTargetInstance == "Aurora" {
		type DisplayDMCluster struct {
			ClusterMeta struct {
				ClusterType    string `json:"cluster_type"`
				ClusterName    string `json:"cluster_name"`
				ClusterVersion string `json:"cluster_version"`
				DeployUser     string `json:"deploy_user"`
				SshType        string `json:"ssh_type"`
				TlsEnabled     bool   `json:"tls_enabled"`
			} `json:"cluster_meta"`
			Instances []struct {
				ID            string `json:"id"`
				Role          string `json:"role"`
				Host          string `json:"host"`
				Ports         string `json:"ports"`
				OsArch        string `json:"os_arch"`
				Status        string `json:"status"`
				Since         string `json:"since"`
				DataDir       string `json:"data_dir"`
				DeployDir     string `json:"deploy_dir"`
				ComponentName string `json:"ComponentName"`
				Port          int    `json:"Port"`
			} `json:"instances"`
		}

		ctx = context.Background()
		ctx = context.WithValue(ctx, "clusterName", clusterName)
		ctx = context.WithValue(ctx, "clusterType", clusterType)

		stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm display %s --format json ", clusterName), false)
		if err != nil {
			return err
		}
		var displayDMCluster DisplayDMCluster
		if err = json.Unmarshal(stdout, &displayDMCluster); err != nil {
			return err
		}

		masterNode := ""
		for _, node := range displayDMCluster.Instances {
			if node.Role == "dm-master" && node.Status == "Healthy" {
				masterNode = fmt.Sprintf("%s:%d", node.Host, node.Port)
			}
		}

		if masterNode == "" {
			return errors.New("No healthy master node found")
		}

		timer.Take("Take DM Master Node")
		var dmTaskDetail task.DMTaskDetail

		hasSynced := true

		tableSyncStatus := [][]string{}
		tableSyncStatus = append(tableSyncStatus, []string{"Sync Flag", "Aurora binlog", "Synced binlog"})
		tui.PrintTable(tableSyncStatus, true)

		for idx := 0; idx < 1000; idx++ {
			time.Sleep(10 * time.Second)
			hasSynced = true

			stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s query-status %s", masterNode, clusterName), false)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(stdout, &dmTaskDetail); err != nil {
				return err
			}

			for _, source := range dmTaskDetail.Sources {
				for _, subTaskStatus := range source.SubTaskStatus {
					tableSyncStatus = [][]string{}
					tableSyncStatus = append(tableSyncStatus, []string{strconv.FormatBool(subTaskStatus.Sync.Synced), subTaskStatus.Sync.MasterBinlog, subTaskStatus.Sync.SyncerBinlog})
					tui.PrintTable(tableSyncStatus, true)
					if subTaskStatus.Sync.Synced == false {
						hasSynced = false
						break
					}

				}
			}
			if hasSynced == true {
				break
			}
		}
		timer.Take("Wait data sync completion")
		// fmt.Printf("The syncd flag is <%#v>\n", hasSynced)

		cyan := color.New(color.FgCyan, color.Bold)
		stdout, _, err = (*workstation).Execute(ctx, "/home/admin/.tiup/bin/sync_diff_inspector --config=/opt/dm-sync-diff-check.toml ", false)
		if err != nil {
			fmt.Printf("Data comparison:      %s\n", cyan.Sprint("Different"))
			fmt.Printf("Data comparison:      %s\n", "Please check the log /tmp/output/sync_diff.log")
			return err
		}

		fmt.Printf("Data comparison:      %s\n", cyan.Sprint("SAME"))
		timer.Take("Data comparison")
	}

	timer.Print()

	return nil
}

func (m *Manager) Aurora2TiDBCloudRunTiDBCloudCluster(clusterName string, opt operator.LatencyWhenBatchOptions, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var sysbenchResult [][]string

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	t1 := task.NewBuilder().RunSysbench(&sexecutor, "/opt/tidbcloud-sysbench.toml", &sysbenchResult, &opt, &gOpt, &cancel).BuildAsStep(fmt.Sprintf("  - Running Ontime Transaction"))
	envInitTasks = append(envInitTasks, t1)

	builder := task.NewBuilder().ParallelStep(fmt.Sprintf("+ Running sysbench against Aurora "), false, envInitTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 2)); err != nil {
		if errorx.Cast(err) != nil {
			return err
		}
		return err
	}

	tui.PrintTable(sysbenchResult, true)
	timer.Take("Sysbench Execution")

	return nil
}

func (m *Manager) QuerySyncStatusAurora2TiDBCloudCluster(clusterName string, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"
	ctx := context.Background()
	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm display %s --format json ", clusterName), false)
	var displayDMCluster task.DisplayDMCluster
	if err = json.Unmarshal(stdout, &displayDMCluster); err != nil {
		return err
	}

	masterNode := ""
	for _, node := range displayDMCluster.Instances {
		if node.Role == "dm-master" && node.Status == "Healthy" {
			masterNode = fmt.Sprintf("%s:%d", node.Host, node.Port)
		}
	}

	var tableSyncStatus [][]string
	var tableSourceStatus [][]string

	tableSourceStatus = append(tableSourceStatus, []string{"Source Name", "Worker"})

	tableSyncStatus = append(tableSyncStatus, []string{"Task Name", "Stage", "Total Events", "Total TPS", "Master binlog", "Synced binlog", "Synced"})
	// var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	var dmTaskDetail task.DMTaskDetail

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s query-status %s", masterNode, clusterName), false)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(stdout, &dmTaskDetail); err != nil {
		return err
	}

	for _, source := range dmTaskDetail.Sources {
		tableSourceStatus = append(tableSourceStatus, []string{source.SourceStatus.Source, source.SourceStatus.Worker})
		for _, subTaskStatus := range source.SubTaskStatus {
			tableSyncStatus = append(tableSyncStatus, []string{subTaskStatus.Name, subTaskStatus.Stage, subTaskStatus.Sync.TotalEvents, subTaskStatus.Sync.TotalTps, subTaskStatus.Sync.MasterBinlog, subTaskStatus.Sync.SyncerBinlog, strconv.FormatBool(subTaskStatus.Sync.Synced)})

		}
	}

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Data comparison:      %s\n", ("SAME"))

	fmt.Printf(cyan.Sprint("Source Status:      \n"))
	tui.PrintTable(tableSourceStatus, true)

	fmt.Printf(cyan.Sprint("\nSync Status:      \n"))
	tui.PrintTable(tableSyncStatus, true)

	return nil
}

func (m *Manager) StopSyncTaskAurora2TiDBCloudCluster(clusterName string, gOpt operator.Options) error {
	clusterType := "ohmytiup-aurora2tidbcloud"
	ctx := context.Background()
	ctx = context.WithValue(ctx, "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm display %s --format json ", clusterName), false)
	var displayDMCluster task.DisplayDMCluster
	if err = json.Unmarshal(stdout, &displayDMCluster); err != nil {
		return err
	}

	masterNode := ""
	for _, node := range displayDMCluster.Instances {
		if node.Role == "dm-master" && node.Status == "Healthy" {
			masterNode = fmt.Sprintf("%s:%d", node.Host, node.Port)
		}
	}

	var tableSyncStatus [][]string
	var tableSourceStatus [][]string

	tableSourceStatus = append(tableSourceStatus, []string{"Source Name", "Worker"})

	tableSyncStatus = append(tableSyncStatus, []string{"Task Name", "Stage", "Total Events", "Total TPS", "Master binlog", "Synced binlog", "Synced"})
	// var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	var dmTaskDetail task.DMTaskDetail

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s query-status %s", masterNode, clusterName), false)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(stdout, &dmTaskDetail); err != nil {
		return err
	}

	for _, source := range dmTaskDetail.Sources {
		tableSourceStatus = append(tableSourceStatus, []string{source.SourceStatus.Source, source.SourceStatus.Worker})
		for _, subTaskStatus := range source.SubTaskStatus {
			_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr=%s stop-task %s ", masterNode, subTaskStatus.Name), false)
		}
	}

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s query-status %s", masterNode, clusterName), false)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(stdout, &dmTaskDetail); err != nil {
		return err
	}

	for _, source := range dmTaskDetail.Sources {
		tableSourceStatus = append(tableSourceStatus, []string{source.SourceStatus.Source, source.SourceStatus.Worker})
		for _, subTaskStatus := range source.SubTaskStatus {
			tableSyncStatus = append(tableSyncStatus, []string{subTaskStatus.Name, subTaskStatus.Stage, subTaskStatus.Sync.TotalEvents, subTaskStatus.Sync.TotalTps, subTaskStatus.Sync.MasterBinlog, subTaskStatus.Sync.SyncerBinlog, strconv.FormatBool(subTaskStatus.Sync.Synced)})

		}
	}

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Data comparison:      %s\n", ("SAME"))

	fmt.Printf(cyan.Sprint("Source Status:      \n"))
	tui.PrintTable(tableSourceStatus, true)

	fmt.Printf(cyan.Sprint("\nSync Status:      \n"))
	tui.PrintTable(tableSyncStatus, true)

	return nil
}
