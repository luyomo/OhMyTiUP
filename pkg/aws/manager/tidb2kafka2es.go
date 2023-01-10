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
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// Deploy a cluster.
func (m *Manager) TiDB2Kafka2ESDeploy(
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

	if err := m.confirmKafkaTopology(name, topo); err != nil {
		return err
	}

	// Setup the execution plan
	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment

	globalOptions := base.GlobalOptions

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var workstationInfo, clusterInfo, kafkaClusterInfo, eksClusterInfo task.ClusterInfo
	// var workstationInfo, eksClusterInfo task.ClusterInfo

	t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	envInitTasks = append(envInitTasks, t1)

	//Setup the kafka cluster
	t2 := task.NewBuilder().CreateKafkaCluster(&sexecutor, "kafka", base.AwsKafkaTopoConfigs, &kafkaClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing kafka servers"))
	envInitTasks = append(envInitTasks, t2)

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV[0].Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t3 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t3)
	}

	t4 := task.NewBuilder().CreateEKSCluster(&sexecutor, base.AwsWSConfigs, base.AwsESTopoConfigs, "es", &eksClusterInfo).
		CreateK8SESCluster(&sexecutor, base.AwsWSConfigs, base.AwsESTopoConfigs, "es", &eksClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing eks servers"))
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
		CreateTransitGatewayVpcAttachment(&sexecutor, "es").
		CreateRouteTgw(&sexecutor, "workstation", []string{"kafka", "tidb", "es"}).
		CreateRouteTgw(&sexecutor, "kafka", []string{"tidb", "es"}).
		CreateRouteTgw(&sexecutor, "es", []string{"workstation"}).
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
func (m *Manager) DestroyTiDB2Kafka2ESCluster(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {

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

	// gOpt.SSHUser, gOpt.IdentityFile
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var destroyTasks []*task.StepDisplay

	t0 := task.NewBuilder().
		DestroyK8SESCluster(&sexecutor, gOpt).
		DestroyEKSCluster(&sexecutor, gOpt).
		DestroyTransitGateways(&sexecutor).
		// DestroyVpcPeering(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	destroyTasks = append(destroyTasks, t0)

	// builder := task.NewBuilder().
	// 	ParallelStep("+ Destroying kafka solution service ... ...", false, t0)
	// t := builder.Build()

	// if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	t1 := task.NewBuilder().
		DestroyNAT(&sexecutor, "kafka").
		DestroyEC2Nodes(&sexecutor, "kafka").
		BuildAsStep(fmt.Sprintf("  - Destroying kafka nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	// t2 := task.NewBuilder().
	// 	DestroyNAT(&sexecutor, "es").
	// 	DestroyEC2Nodes(&sexecutor, "es").
	// 	BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

	// destroyTasks = append(destroyTasks, t2)

	t4 := task.NewBuilder().
		DestroyNAT(&sexecutor, "tidb").
		DestroyEC2Nodes(&sexecutor, "tidb").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation and tidb cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	builder := task.NewBuilder().
		ParallelStep("+ Destroying all the componets", false, destroyTasks...)

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

	t10 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Removing workstation"))

	t10.Execute(ctxt.New(tailctx, 1))

	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListTiDB2Kafka2ESCluster(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

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

// type MapTiDB2PG struct {
// 	TiDB2PG []struct {
// 		TiDB struct {
// 			DataType string   `yaml:"DataType"`
// 			Def      string   `yaml:"Def"`
// 			Queries  []string `yaml:"Query,omitempty"`
// 		} `yaml:"TiDB"`
// 		PG struct {
// 			DataType string   `yaml:"DataType"`
// 			Def      string   `yaml:"Def"`
// 			Queries  []string `yaml:"Query,omitempty"`
// 		} `yaml:"PG"`
// 		Value string `yaml:"Value"`
// 	} `yaml:"MapTiDB2PG"`
// }

/*
	*****************************************************************************

Parameters:

	perfOpt
	  -> DataTypeDtr: ["int", "varchar"]
*/
func (m *Manager) PerfPrepareTiDB2Kafka2ES(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {
	/* ********** ********** 003. Prepare execution context **********/
	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
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

	/* ********** ********** 004 Prepare insert query to /opt/kafka/query.sql **********/
	strInsQuery := fmt.Sprintf("insert into test.test01(col02) values(1)")
	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("echo \\\"%s\\\" > /tmp/query.sql", strInsQuery), true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/query.sql /opt/kafka/", true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, "chmod 777 /opt/kafka/query.sql", true); err != nil {
		return err
	}

	// 006.01 Reset test01 test table
	commands := []string{
		"drop table if exists test01",
		fmt.Sprintf("create table test01(col01 bigint auto_random primary key, col02 int)"),
	}

	for _, command := range commands {
		_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query test '%s'", command), false, 1*time.Hour)
		if err != nil {
			return err
		}
	}

	timer.Take("03. Table creation in the TiDB")

	/* ********** ********** 007 Prepare kafka related objects  **********/

	// 007.02 Script create topic for multiple partition in advanced.
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %s %d", "test_test01", perfOpt.Partitions), false, 1*time.Hour)
	if err != nil {
		return err
	}

	timer.Take("04. Create kafka topic in advanced for multiple parations per table - /opt/kafka/source.toml")

	/* ********** ********** 008 Extract server info(ticdc/broker/schema registry/ connector)   **********/
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t1)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var cdcIP, schemaRegistryIP, brokerIP, connectorIP string
	for _, row := range tableECs {
		if row[0] == "ticdc" {
			cdcIP = row[5]
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
	timer.Take("05. Get required info - pd/broker/schemaRegistry/connector")

	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --server http://%s:8300 2>/dev/null", cdcIP), false)
	if err != nil {
		return err
	}

	/* ********** ********** 009 Prepare TiCDC source changefeed   **********/
	// 008.01 TiCDC source config file
	if err = (*workstation).TransferTemplate(ctx, "templates/config/ticdc.source.toml.tpl", "/opt/kafka/source.toml", "0644", []string{}, true, 0); err != nil {
		return err
	}

	// 008.02 Extract changefeed status
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

	// 009.03 Create changefeed if it does not exists
	if changeFeedHasExisted == false {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed create --server http://%s:8300 --changefeed-id='%s' --sink-uri='kafka://%s:9092/%s?protocol=avro' --schema-registry=http://%s:8081 --config %s", cdcIP, "kafka-avro", brokerIP, "topic-name", schemaRegistryIP, "/opt/kafka/source.toml"), false); err != nil {
			return err
		}
	}

	timer.Take("06. Create if not exists changefeed of TiCDC")
	fmt.Printf("The connectot ip is <%s> \n\n\n", connectorIP)

	esConfig := make(map[string]string)
	esConfig["ES_IP"] = "192.168.1.1"
	esConfig["ES_User"] = "elastic"
	esConfig["ES_Password"] = "1234Abcd"
	esConfig["SchemaRegistry"] = schemaRegistryIP
	esConfig["TopicName"] = "test_test01"
	if err = (*workstation).TransferTemplate(ctx, "templates/config/tidb2kafka2es/kafka.sink.json", "/opt/kafka/es.sink.json", "0644", esConfig, true, 0); err != nil {
		return err
	}

	// if _, _, err := (*workstation).Execute(ctx, "mv /tmp/kafka.sink.json /opt/kafka/", true); err != nil {
	// 	return err
	// }

	// if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/kafka.sink.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
	// 	return err
	// }
	// timer.Take("07. Create the kafka sink to postgres")

	timer.Print()

	return nil
}

func (m *Manager) PerfTiDB2Kafka2ES(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
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
	timer.Take("TiDB Conn info")

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

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("mysqlslap --no-defaults -h %s -P %d --user=%s --query=/opt/kafka/query.sql --concurrency=%d --iterations=%d --number-of-queries=%d --create='drop database if exists test02;create schema test02' --no-drop", tidbConnectInfo.TiDBHost, tidbConnectInfo.TiDBPort, tidbConnectInfo.TiDBUser, 10, 1, perfOpt.NumOfRecords), true, 1*time.Hour)
	if err != nil {
		return err
	}
	timer.Take("mysqlslap running")

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

	timer.Take("Wait until data sync completion")

	// 05. Calculate the QPS and latency
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	cnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*)/(EXTRACT(EPOCH FROM max(tidb_timestamp) - min(tidb_timestamp))::int + 1) from test01"), false, 1*time.Hour)
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

func (m *Manager) PerfCleanTiDB2Kafka2ES(clusterName, clusterType string, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	// Get executor
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
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

	var cdcIP, connectorIP, schemaRegistryIP string
	for _, row := range tableECs {
		if row[0] == "ticdc" {
			cdcIP = row[5]
		}

		if row[0] == "connector" {
			connectorIP = row[5]
		}
		if row[0] == "schemaRegistry" {
			schemaRegistryIP = row[5]
		}

	}

	// Remove the sink connector
	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8083/connectors/%s", connectorIP, "JDBCTEST"), false); err != nil {
		return err
	}

	// Remove the changefeed
	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --server http://%s:8300 2>/dev/null", cdcIP), false)
	if err != nil {
		return err
	}
	changeFeedExist, err := changeFeedExist(stdout, "avro-test")
	if err != nil {
		return err
	}
	if changeFeedExist == true {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed remove --server http://%s:8300 --changefeed-id='%s'", cdcIP, "kafka-avro"), false); err != nil {
			return err
		}
	}

	// Remove the topic

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, schemaRegistryIP, "sudo systemctl stop confluent-schema-registry"), false, 600*time.Second); err != nil {
		return err
	}

	for _, topicName := range []string{"topic-name", "test_test01", "_schemas"} {
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka-util.sh remove-topic %s", topicName), false); err != nil {
			logger.OutputDebugLog(err.Error())
			//return err
		}
	}
	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, schemaRegistryIP, "sudo systemctl start confluent-schema-registry"), false, 600*time.Second); err != nil {
		return err
	}

	// Remove the Postgres db and table
	// _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "drop database if exists test"), false, 1*time.Hour)
	// if err != nil {
	// 	return err
	// }

	// Remove the TiDB db and table

	return nil
}

// func changeFeedExist(inputStr []byte, changeFeedID string) (bool, error) {
// 	type ChangeFeedSummary struct {
// 		State      string `json:"state"`
// 		Tso        int    `json:"tso"`
// 		Checkpoint string `json:"checkpoint"`
// 		Error      string `json:"error"`
// 	}

// 	type ChangeFeed struct {
// 		Id string `json:"id"`
// 		// Summary ChangeFeedSummary `json:"summary"`
// 		Summary struct {
// 			State      string `json:"state"`
// 			Tso        int    `json:"tso"`
// 			Checkpoint string `json:"checkpoint"`
// 			Error      string `json:"error"`
// 		} `json:"summary"`
// 	}
// 	var changeFeeds []ChangeFeed

// 	err := yaml.Unmarshal(inputStr, &changeFeeds)
// 	if err != nil {
// 		return false, err
// 	}

// 	for _, changeFeed := range changeFeeds {
// 		if changeFeed.Id == changeFeedID {
// 			return true, nil
// 		}
// 	}
// 	return false, nil

// }

// ***************************************** Postgres 2 TiDB **********************************************

// type MapPG2TiDB struct {
// 	PG2TiDB []struct {
// 		TiDB struct {
// 			DataType string   `yaml:"DataType"`
// 			Def      string   `yaml:"Def"`
// 			Queries  []string `yaml:"Query,omitempty"`
// 		} `yaml:"TiDB"`
// 		PG struct {
// 			DataType string   `yaml:"DataType"`
// 			Def      string   `yaml:"Def"`
// 			Queries  []string `yaml:"Query,omitempty"`
// 		} `yaml:"PG"`
// 		Value string `yaml:"Value"`
// 	} `yaml:"MapPG2TiDB"`
// }

/*
	*****************************************************************************

Parameters:

	perfOpt
	  -> DataTypeDtr: ["int", "varchar"]
*/
// func (m *Manager) PerfPreparePG2Kafka2TiDB(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

// 	/* ********** ********** 001. Read the column mapping file to struct
// 		   ColumnMapping.yml:
// 		   MapTiDB2PG:
// 		   - TiDB:
// 		       DataType: BOOL
// 		       Def: t_bool BOOL
// 		     PG:
// 		       DataType: BOOL
// 		       Def: t_bool BOOL
// 		    Value: true
// 		    ... ...
// 	           - TiDB:
// 	               DataType: SET
// 	               Def: t_set SET('"'"'a'"'"','"'"'b'"'"','"'"'c'"'"')
// 	             PG:
// 	               DataType: ENUM
// 	               Def: t_set t_enum_test[]
// 	             Query:
// 	               - create type t_enum_test as enum  ('"'"'a'"'"','"'"'b'"'"','"'"'c'"'"')
// 	*/
// 	mapFile, err := ioutil.ReadFile("embed/templates/config/pg2kafka2tidb/ColumnMapping.yml")
// 	if err != nil {
// 		return err
// 	}

// 	var mapPG2TiDB MapPG2TiDB
// 	err = yaml.Unmarshal(mapFile, &mapPG2TiDB)
// 	if err != nil {
// 		return err
// 	}

// 	/* ********** ********** 002. Prepare columns defintion to be executed into TiDB and postgres ********** ********** */
// 	var arrTiDBTblDataDef []string // Array to keep tidb column definition. ex: ["pk_col BIGINT PRIMARY KEY AUTO_RANDOM", ... , "tidb_timestamp timestamp default current_timestamp"]
// 	var arrPGTblDataDef []string   // Array to keep postgres column definition. ex: ["pk_col bigint PRIMARY KEY", ... ... "tidb_timestamp timestamp", "pg_timestamp timestamp default current_timestamp"]
// 	var arrCols []string           // Array to keep all column names. ex: ["t_bool"]
// 	var arrData []string           // Array to keep data to be inserted. ex: ["true"]
// 	var pgPreQueries []string      // Array of queries to be executed in PG. ex: ["create type t_enum_test ..."]

// 	/* 002.01 Prepare primary key column definition */
// 	arrPGTblDataDef = append(arrPGTblDataDef, "pk_col BIGSERIAL PRIMARY KEY")
// 	arrTiDBTblDataDef = append(arrTiDBTblDataDef, "pk_col BIGINT PRIMARY KEY")

// 	/* 002.02 Prepare column definition body */
// 	for _, _dataType := range perfOpt.DataTypeDtr {
// 		for _, _mapItem := range mapPG2TiDB.PG2TiDB {
// 			if _dataType == _mapItem.PG.DataType {
// 				arrPGTblDataDef = append(arrPGTblDataDef, _mapItem.PG.Def)
// 				arrTiDBTblDataDef = append(arrTiDBTblDataDef, _mapItem.TiDB.Def)
// 				arrCols = append(arrCols, strings.Split(_mapItem.PG.Def, " ")[0])
// 				arrData = append(arrData, strings.Replace(strings.Replace(_mapItem.Value, "<<<<", "'", 1), ">>>>", "'", 1))
// 				pgPreQueries = append(pgPreQueries, _mapItem.PG.Queries...)
// 			}
// 		}
// 	}

// 	/* 002.03 Prepare tail columns for both TiDB and postgres tables.*/
// 	arrPGTblDataDef = append(arrPGTblDataDef, "pg_timestamp timestamp default current_timestamp")

// 	arrTiDBTblDataDef = append(arrTiDBTblDataDef, "pg_timestamp timestamp(6)")
// 	arrTiDBTblDataDef = append(arrTiDBTblDataDef, "tidb_timestamp timestamp(6) default current_timestamp(6)")

// 	/* ********** ********** 003. Prepare execution context **********/
// 	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
// 	ctx = context.WithValue(ctx, "clusterType", clusterType)

// 	var timer awsutils.ExecutionTimer
// 	timer.Initialize([]string{"Step", "Duration(s)"})
// 	// 01. Get the workstation executor
// 	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
// 	if err != nil {
// 		return err
// 	}

// 	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
// 	if err != nil {
// 		return err
// 	}

// 	_, _, err = (*workstation).Execute(ctx, "apt-get install -y postgresql-contrib", true, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}

// 	/* ********** ********** 004 Prepare insert query to /opt/kafka/query.sql **********/
// 	strInsQuery := fmt.Sprintf("insert into test.test01(%s) values(%s)", strings.Join(arrCols, ","), strings.Join(arrData, ","))
// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("echo \\\"%s\\\" > /tmp/query.sql", strInsQuery), true); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/query.sql /opt/kafka/", true); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, "chmod 777 /opt/kafka/query.sql", true); err != nil {
// 		return err
// 	}

// 	/* ********** ********** 005 Prepare postgres objects  **********/
// 	// 005.01 Reset test database if exists
// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "drop database if exists test"), false, 1*time.Hour); err != nil {
// 		return err
// 	}

// 	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query postgres '%s'", "create database test"), false, 1*time.Hour); err != nil {
// 		return err
// 	}

// 	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query test '%s'", "create schema test"), false, 1*time.Hour); err != nil {
// 		return err
// 	}
// 	timer.Take("01. Postgres DB creation")

// 	// 005.02 Create postgres objects for test. Like enum
// 	for _, query := range pgPreQueries {
// 		fmt.Printf("the query is <%s> \n\n\n", query)
// 		_, stderr, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query test '%s'", query), false, 1*time.Hour)
// 		if err != nil {
// 			logger.OutputDebugLog(string(stderr))
// 			return err
// 		}
// 	}

// 	// 005. 03 Create test table
// 	commands := []string{
// 		fmt.Sprintf("create table test.test01(%s)", strings.Join(arrPGTblDataDef, ",")),
// 	}

// 	for _, command := range commands {
// 		_, stderr, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query test '%s'", command), false, 1*time.Hour)
// 		if err != nil {
// 			logger.OutputDebugLog(string(stderr))
// 			return err
// 		}
// 	}
// 	timer.Take("02. Table Creation in the postgres")

// 	/* ********** ********** 006 Prepare postgres objects  **********/
// 	// 006.01 Reset test01 test table
// 	commands = []string{
// 		"drop table if exists test01",
// 		fmt.Sprintf("create table test01(%s)", strings.Join(arrTiDBTblDataDef, ",")),
// 	}

// 	for _, command := range commands {
// 		if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query test '%s'", command), false, 1*time.Hour); err != nil {
// 			return err
// 		}
// 	}

// 	timer.Take("03. Table creation in the TiDB")

// 	/* ********** ********** 007 Prepare kafka related objects  **********/

// 	timer.Take("04. Create kafka topic in advanced for multiple parations per table - /opt/kafka/source.toml")

// 	/* ********** ********** 008 Extract server info(ticdc/broker/schema registry/ connector)   **********/
// 	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
// 	var tableECs [][]string
// 	t1 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
// 	listTasks = append(listTasks, t1)

// 	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

// 	t := builder.Build()

// 	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
// 		return err
// 	}

// 	var schemaRegistryIP, connectorIP string
// 	for _, row := range tableECs {
// 		if row[0] == "schemaRegistry" {
// 			schemaRegistryIP = row[5]
// 		}
// 		if row[0] == "connector" {
// 			connectorIP = row[5]
// 		}
// 	}
// 	timer.Take("05. Get required info - pd/broker/schemaRegistry/connector")

// 	type PGSourceData struct {
// 		PGHost         string `yaml:"Host"`
// 		PGPort         int    `yaml:"Port"`
// 		PGUser         string `yaml:"User"`
// 		PGPassword     string `yaml:"Password"`
// 		PGDBName       string
// 		SchemaRegistry string
// 	}

// 	pgSourceData := PGSourceData{}
// 	if err = task.ReadDBConntionInfo(workstation, "db-info.yml", &pgSourceData); err != nil {
// 		return err
// 	}

// 	pgSourceData.SchemaRegistry = schemaRegistryIP
// 	pgSourceData.PGDBName = "test"

// 	// 008.01 TiCDC source config file
// 	if err = (*workstation).TransferTemplate(ctx, "templates/config/pg2kafka2tidb/source.pg.tpl.json", "/tmp/source.pg.json", "0644", pgSourceData, true, 0); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/source.pg.json /opt/kafka/", true); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/source.pg.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
// 		return err
// 	}

// 	type TiDBSinkData struct {
// 		TiDBHost       string `yaml:"Host"`
// 		TiDBPort       int    `yaml:"Port"`
// 		TiDBUser       string `yaml:"User"`
// 		TiDBPassword   string `yaml:"Password"`
// 		TiDBDBName     string
// 		SchemaRegistry string
// 	}
// 	var tidbSinkData TiDBSinkData

// 	err = task.ReadDBConntionInfo(workstation, "tidb-db-info.yml", &tidbSinkData)

// 	tidbSinkData.TiDBDBName = "test"
// 	tidbSinkData.SchemaRegistry = schemaRegistryIP
// 	if err = (*workstation).TransferTemplate(ctx, "templates/config/pg2kafka2tidb/sink.tidb.tpl.json", "/opt/kafka/sink.tidb.json", "0644", tidbSinkData, true, 0); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/sink.tidb.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
// 		return err
// 	}

// 	return nil

// }

// func (m *Manager) PerfPG2Kafka2TiDB(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

// 	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
// 	ctx = context.WithValue(ctx, "clusterType", clusterType)

// 	var timer awsutils.ExecutionTimer
// 	timer.Initialize([]string{"Step", "Duration(s)"})

// 	// 01. Get the workstation executor
// 	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
// 	if err != nil {
// 		return err
// 	}

// 	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
// 	if err != nil {
// 		return err
// 	}

// 	// 02. Get the TiDB connection info
// 	type PGConnectInfo struct {
// 		PGHost     string `yaml:"Host"`
// 		PGPort     int    `yaml:"Port"`
// 		PGUser     string `yaml:"User"`
// 		PGPassword string `yaml:"Password"`
// 	}

// 	var pgConnectInfo PGConnectInfo

// 	if err = task.ReadDBConntionInfo(workstation, "db-info.yml", &pgConnectInfo); err != nil {
// 		return err
// 	}

// 	timer.Take("PG Conn info")

// 	// 03. Prepare the query to insert data into TiDB
// 	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "truncate table test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}

// 	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "truncate table test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}

// 	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf("PGPASSWORD=%s pgbench -f /opt/kafka/query.sql -h %s -p %d -U %s -d test -j%d -T %d", pgConnectInfo.PGPassword, pgConnectInfo.PGHost, pgConnectInfo.PGPort, pgConnectInfo.PGUser, 10, 10), true, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}
// 	timer.Take("pgbench running")

// 	// 04. Wait the data sync to postgres
// 	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}
// 	pgCnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

// 	for cnt := 0; cnt < 20; cnt++ {
// 		time.Sleep(10 * time.Second)
// 		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select count(*) from test.test01"), false, 1*time.Hour)
// 		if err != nil {
// 			return err
// 		}
// 		tidbCnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

// 		if pgCnt == tidbCnt {
// 			break
// 		}
// 	}

// 	timer.Take("Wait until data sync completion")

// 	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select round(count(*)/((max(pg_timestamp) - min(pg_timestamp)) + 1)) from test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}
// 	tidbQPS := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

// 	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select floor(sum(tidb_timestamp - pg_timestamp)/count(*)) from test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}
// 	latency := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

// 	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select round(count(*)/(max(tidb_timestamp) - min(pg_timestamp))) as min_pg from test.test01"), false, 1*time.Hour)
// 	if err != nil {
// 		return err
// 	}
// 	qps := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

// 	perfMetrics := [][]string{{"Count", "PG DB QPS", "PG 2 TiDB Latency", "PG 2 TiDB QPS"}}
// 	perfMetrics = append(perfMetrics, []string{pgCnt, tidbQPS, latency, qps})

// 	tui.PrintTable(perfMetrics, true)

// 	return nil
// }

// func (m *Manager) PerfCleanPG2Kafka2TiDB(clusterName, clusterType string, gOpt operator.Options) error {

// 	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
// 	ctx = context.WithValue(ctx, "clusterType", clusterType)

// 	// Get executor
// 	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
// 	if err != nil {
// 		return err
// 	}

// 	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
// 	if err != nil {
// 		return err
// 	}

// 	// Get server info
// 	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
// 	var tableECs [][]string
// 	t1 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
// 	listTasks = append(listTasks, t1)

// 	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

// 	t := builder.Build()

// 	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
// 		return err
// 	}

// 	var connectorIP, schemaRegistryIP, brokerIP string
// 	for _, row := range tableECs {
// 		if row[0] == "broker" {
// 			brokerIP = row[5]
// 		}
// 		if row[0] == "connector" {
// 			connectorIP = row[5]
// 		}
// 		if row[0] == "schemaRegistry" {
// 			schemaRegistryIP = row[5]
// 		}

// 	}

// 	// Remove the sink connector
// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8083/connectors/%s", connectorIP, "SINKTiDB"), false); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8083/connectors/%s", connectorIP, "sourcepg"), false); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8081/subjects/sourcepg.test.test01-key", schemaRegistryIP), false); err != nil {
// 		return err
// 	}

// 	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8081/subjects/sourcepg.test.test01-value", schemaRegistryIP), false); err != nil {
// 		return err
// 	}

// 	for _, _topic := range []string{"sourcepg.test.test01", "_schemas", "__offset_topics"} {
// 		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("kafka-topics --delete --bootstrap-server http://%s:9092 --topic %s", brokerIP, _topic), false); err != nil {
// 			fmt.Printf("Error: %#v", err)
// 			//		return err
// 		}
// 	}

// 	return nil
// }
