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
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	// "github.com/luyomo/tisample/pkg/aws/clusterutil"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	awsutils "github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/logger"
	// "github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
	// "os"
)

// DeployOptions contains the options for scale out.
type TiDB2Kafka2PgDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) TiDB2Kafka2PgDeploy(
	name string,
	topoFile string,
	opt TiDB2Kafka2PgDeployOptions,
	gOpt operator.Options,
) error {
	clusterType := "ohmytiup-tidb2kafka2pg"

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

	if err := m.confirmTopology(name, "v5.1.0", topo, set.NewStringSet()); err != nil {
		return err
	}

	// Setup the execution plan
	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	globalOptions := base.GlobalOptions

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var workstationInfo, clusterInfo, kafkaClusterInfo, pgClusterInfo task.ClusterInfo
	// var workstationInfo, clusterInfo task.ClusterInfo

	t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	envInitTasks = append(envInitTasks, t1)

	//Setup the kafka cluster
	t2 := task.NewBuilder().CreateKafkaCluster(&sexecutor, "kafka", base.AwsKafkaTopoConfigs, &kafkaClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing kafka servers"))
	envInitTasks = append(envInitTasks, t2)

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV.Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t3 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t3)
	}

	t4 := task.NewBuilder().
		CreatePostgres(&sexecutor, base.AwsWSConfigs, base.AwsPostgresConfigs, &pgClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing postgres servers"))
	envInitTasks = append(envInitTasks, t4)

	builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for kafka solution service", false, envInitTasks...)

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
		CreateTransitGatewayVpcAttachment(&sexecutor, "kafka").
		CreateTransitGatewayVpcAttachment(&sexecutor, "tidb").
		CreateTransitGatewayVpcAttachment(&sexecutor, "postgres").
		CreateRouteTgw(&sexecutor, "workstation", []string{"kafka", "tidb", "postgres"}).
		CreateRouteTgw(&sexecutor, "kafka", []string{"tidb", "postgres"}).
		DeployKafka(&sexecutor, base.AwsWSConfigs, "kafka", &workstationInfo).
		DeployTiDB(&sexecutor, "tidb", base.AwsWSConfigs, &workstationInfo).
		DeployTiDBInstance(&sexecutor, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Prepare network resources %s:%d", globalOptions.Host, 22))

	builder = task.NewBuilder().
		ParallelStep("+ Deploying kafka solution service ... ...", false, t5)
	t = builder.Build()

	timer.Take("Preparation")
	if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
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
func (m *Manager) DestroyTiDB2Kafka2PgCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	clusterType := "ohmytiup-tidb2kafka2pg"

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
		DestroyVpcPeering(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().
		ParallelStep("+ Destroying kafka solution service ... ...", false, t0)
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
		DestroyNAT(&sexecutor, "kafka").
		DestroyEC2Nodes(&sexecutor, "kafka").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	t4 := task.NewBuilder().
		DestroyNAT(&sexecutor, "tidb").
		DestroyEC2Nodes(&sexecutor, "workstation").
		DestroyEC2Nodes(&sexecutor, "tidb").
		DestroyPostgres(&sexecutor).
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

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListTiDB2Kafka2PgCluster(clusterName string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb2kafka2pg")

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

func (m *Manager) PerfPrepareTiDB2Kafka2PG(clusterName string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb2kafka2pg")

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, "ohmytiup-tidb2kafka2pg", gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	// 02. Create the postgres objects(Database and tables)
	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "drop database if exists test"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	// fmt.Printf("The outpur from the query is <%s> \n\n\n", stdout)

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "create database  test"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	commands := []string{
		"create table test01(col01 bigint PRIMARY KEY, col02 int, tidb_timestamp timestamp, pg_timestamp timestamp default current_timestamp)",
	}

	for _, command := range commands {
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query test '%s'", command), false, 1*time.Hour)
		if err != nil {
			return err
		}
	}

	// 03. Create TiDB objects(Databse and tables)
	commands = []string{
		"drop table if exists test01",
		"create table test01(col01 bigint PRIMARY KEY AUTO_INCREMENT, col02 int , tidb_timestamp timestamp default current_timestamp)",
	}

	for _, command := range commands {
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query test '%s'", command), false, 1*time.Hour)
		if err != nil {
			return err
		}
	}

	// 04. Deploy the ticdc source connector
	err = (*workstation).TransferTemplate(ctx, "templates/config/tidb2kafka2pg/source.toml.tpl", "/tmp/source.toml", "0644", []string{}, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/source.toml /opt/kafka/", true); err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %s %d", "test_test01", perfOpt.Partitions), false, 1*time.Hour)
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

	var pdIP, schemaRegistryIP, brokerIP, connectorIP string
	for _, row := range tableECs {
		if row[0] == "pd" {
			pdIP = row[5]
		}
		if row[0] == "broker" {
			brokerIP = row[5]
		}
		if row[0] == "schemaRegistry" {
			schemaRegistryIP = row[5]
		}
		if row[0] == "connector" {
			connectorIP = row[5]
		}
	}

	if stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --pd=http://%s:2379 2>/dev/null", pdIP), false); err != nil {
		return err
	}

	type ChangeFeed struct {
		Id string `json:"id"`
		// Summary ChangeFeedSummary `json:"summary"`
		Summary struct {
			State      string `json:"state"`
			Tso        int    `json:"tso"`
			Checkpoint string `json:"checkpoint"`
			Error      string `json:"error"`
		} `json:"summary"`
	}
	var changeFeeds []ChangeFeed

	err = yaml.Unmarshal(stdout, &changeFeeds)
	if err != nil {
		return err
	}

	changeFeedHasExisted := false
	for _, changeFeed := range changeFeeds {
		if changeFeed.Id == "kafka-avro" {
			changeFeedHasExisted = true
		}
	}

	if changeFeedHasExisted == false {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed create --pd=http://%s:2379 --changefeed-id='%s' --sink-uri='kafka://%s:9092/%s?protocol=avro' --schema-registry=http://%s:8081 --config %s", pdIP, "kafka-avro", brokerIP, "topic-name", schemaRegistryIP, "/opt/kafka/source.toml"), false); err != nil {
			return err
		}
	}

	// 05. Deploy the sink connector for kafka
	if err = (*workstation).Transfer(ctx, "/opt/db-info.yml", "/tmp/db-info.yml", true, 1024); err != nil {
		return err
	}
	type PGSinkData struct {
		PGHost         string `yaml:"Host"`
		PGPort         int    `yaml:"Port"`
		PGUser         string `yaml:"User"`
		PGPassword     string `yaml:"Password"`
		PGDBName       string
		TopicName      string
		TableName      string
		SchemaRegistry string
	}

	pgSinkData := PGSinkData{}

	yfile, err := ioutil.ReadFile("/tmp/db-info.yml")
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yfile, &pgSinkData)
	if err != nil {
		return err
	}
	pgSinkData.PGDBName = "test"
	pgSinkData.TopicName = "test_test01"
	pgSinkData.TableName = "test01"
	pgSinkData.SchemaRegistry = schemaRegistryIP

	fmt.Printf("The data is <%#v> \n\n\n", pgSinkData)

	err = (*workstation).TransferTemplate(ctx, "templates/config/kafka.sink.json", "/tmp/kafka.sink.json", "0644", pgSinkData, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/kafka.sink.json /opt/kafka/", true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/kafka.sink.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
		return err
	}

	return nil
}

func (m *Manager) PerfTiDB2Kafka2PG(clusterName string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb2kafka2pg")

	// 01. Get the workstation executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, "ohmytiup-tidb2kafka2pg", gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	// 02. Get the TiDB connection info
	if err = (*workstation).Transfer(ctx, "/opt/tidb-db-info.yml", "/tmp/tidb-db-info.yml", true, 1024); err != nil {
		return err
	}
	type TiDBConnectInfo struct {
		TiDBHost     string `yaml:"Host"`
		TiDBPort     int    `yaml:"Port"`
		TiDBUser     string `yaml:"User"`
		TiDBPassword string `yaml:"Password"`
	}

	tidbConnectInfo := TiDBConnectInfo{}

	yfile, err := ioutil.ReadFile("/tmp/tidb-db-info.yml")
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yfile, &tidbConnectInfo)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", "drop database if exists mysqlslap"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	// 03. Prepare the query to insert data into TiDB
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "truncate table test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "truncate table  test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("mysqlslap --no-defaults -h %s -P %d --user=%s --query='%s' --concurrency=%d --iterations=%d --number-of-queries=%d --create='drop database if exists test02;create schema test02' --no-drop", tidbConnectInfo.TiDBHost, tidbConnectInfo.TiDBPort, tidbConnectInfo.TiDBUser, "insert into test.test01(col02) values(1)", 10, 1, perfOpt.NumOfRecords), true, 1*time.Hour)
	if err != nil {
		return err
	}

	// 04. Wait the data sync to postgres
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	tidbCnt := strings.Trim(string(stdout), "\n")

	for cnt := 0; cnt < 20; cnt++ {
		time.Sleep(10 * time.Second)
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
		if err != nil {
			return err
		}
		pgCnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

		if pgCnt == tidbCnt {
			break
		}
	}

	// 05. Calculate the QPS and latency
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	cnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*)/EXTRACT(EPOCH FROM max(tidb_timestamp) - min(tidb_timestamp))::int from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	tidbQPS := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select EXTRACT(EPOCH FROM (sum(pg_timestamp - tidb_timestamp)/count(*)))::int  from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	latency := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select (count(*)/EXTRACT(EPOCH FROM max(pg_timestamp) - min(tidb_timestamp)))::int as min_pg from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	qps := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	perfMetrics := [][]string{{"Count", "DB QPS", "TiDB 2 PG Latency", "TiDB 2 PG QPS"}}
	perfMetrics = append(perfMetrics, []string{cnt, tidbQPS, latency, qps})

	tui.PrintTable(perfMetrics, true)

	return nil
}

func (m *Manager) PerfCleanTiDB2Kafka2PG(clusterName string, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-tidb2kafka2pg")

	// Get executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, "ohmytiup-tidb2kafka2pg", gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	// Get server info
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t1)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var pdIP, connectorIP string
	for _, row := range tableECs {
		if row[0] == "pd" {
			pdIP = row[5]
		}

		if row[0] == "connector" {
			connectorIP = row[5]
		}
	}

	// Remove the sink connector
	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8083/connectors/%s", connectorIP, "JDBCTEST"), false); err != nil {
		return err
	}

	// Remove the changefeed
	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --pd=http://%s:2379 2>/dev/null", pdIP), false)
	if err != nil {
		return err
	}
	changeFeedExist, err := changeFeedExist(stdout, "avro-test")
	if err != nil {
		return err
	}
	if changeFeedExist == true {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed remove --pd=http://%s:2379 --changefeed-id='%s'", pdIP, "kafka-avro"), false); err != nil {
			return err
		}
	}

	// Remove the topic
	for _, topicName := range []string{"topic-name", "test_test01"} {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka-util.sh remove-topic %s", topicName), false); err != nil {
			return err
		}
	}

	// Remove the Postgres db and table
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "drop database if exists test"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	// Remove the TiDB db and table

	return nil
}

func changeFeedExist(inputStr []byte, changeFeedID string) (bool, error) {
	type ChangeFeedSummary struct {
		State      string `json:"state"`
		Tso        int    `json:"tso"`
		Checkpoint string `json:"checkpoint"`
		Error      string `json:"error"`
	}

	type ChangeFeed struct {
		Id string `json:"id"`
		// Summary ChangeFeedSummary `json:"summary"`
		Summary struct {
			State      string `json:"state"`
			Tso        int    `json:"tso"`
			Checkpoint string `json:"checkpoint"`
			Error      string `json:"error"`
		} `json:"summary"`
	}
	var changeFeeds []ChangeFeed

	err := yaml.Unmarshal(inputStr, &changeFeeds)
	if err != nil {
		return false, err
	}

	for _, changeFeed := range changeFeeds {
		if changeFeed.Id == changeFeedID {
			return true, nil
		}
	}
	return false, nil

}
