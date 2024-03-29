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
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	perrs "github.com/pingcap/errors"
	yaml "gopkg.in/yaml.v3"

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

	// globalOptions := base.GlobalOptions

	// -- eks / es ----------------------------------------------------------->
	// -- workstation cluster   | --> routes --> | --> TiDB instance deployment
	// -- tidb cluster          |                | --> kafka insance deployment
	// -- kafka cluster         |

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := m.makeExeContext(ctx, nil, &gOpt, EXC_WS, ws.EXC_AWS_ENV); err != nil {
		return err
	}

	var workstationInfo, clusterInfo, _, _ task.ClusterInfo

	var mainTask []*task.StepDisplay // tasks which are used to initialize environment

	var task001 []*task.StepDisplay // tasks which are used to initialize environment

	fpMakeWSContext := func() error {
		if err := m.makeExeContext(ctx, nil, &gOpt, INC_WS, ws.EXC_AWS_ENV); err != nil {
			return err
		}
		return nil
	}
	t1 := task.NewBuilder().CreateWorkstationCluster(&m.localExe, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt, fpMakeWSContext).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	task001 = append(task001, t1)

	// t2 := task.NewBuilder().CreateKafkaCluster(&sexecutor, "kafka", base.AwsKafkaTopoConfigs, &kafkaClusterInfo).BuildAsStep(fmt.Sprintf("  - Preparing kafka servers"))
	// task001 = append(task001, t2)

	t3 := task.NewBuilder().CreateTiDBCluster(&m.localExe, "tidb", base.AwsTopoConfigs, &clusterInfo).BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	task001 = append(task001, t3)

	t4 := task.NewBuilder().CreateTransitGateway(&m.localExe).BuildAsStep(fmt.Sprintf("  - Preparing the transit gateway"))
	task001 = append(task001, t4)

	// Parallel task to create tidb and kafka instances
	var task002 []*task.StepDisplay // tasks which are used to initialize environment

	// t22 := task.NewBuilder().DeployKafka(&sexecutor, base.AwsWSConfigs, "kafka", &workstationInfo).BuildAsStep(fmt.Sprintf("  - Preparing kafka instance"))
	// task002 = append(task002, t22)

	t23 := task.NewBuilder().
		DeployTiDB("tidb", base.AwsWSConfigs, base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
		DeployTiDBInstance(base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, base.AwsTopoConfigs.General.EnableAuditLog, &m.workstation).
		BuildAsStep(fmt.Sprintf("  - Deploying tidb instance ... "))
	task002 = append(task002, t23)

	// The es might be lag behind the tidb/kafka cluster
	// Cluster generation -> transit gateway setup -> instance deployment
	paraTask001 := task.NewBuilder().ParallelStep("+ Deploying all the sub components for kafka solution service", false, task001...).
		// CreateRouteTgw(&sexecutor, "workstation", []string{"kafka", "tidb", "es"}).
		CreateRouteTgw(&m.localExe, "workstation", []string{"tidb", "es"}).
		CreateRouteTgw(&m.localExe, "kafka", []string{"tidb", "es"}).
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
func (m *Manager) DestroyTiDB2Kafka2ESCluster(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {

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
	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	var destroyTasks []*task.StepDisplay

	t1 := task.NewBuilder().DestroyTransitGateways(&m.localExe).BuildAsStep("  - Removing transit gateway")
	destroyTasks = append(destroyTasks, t1)

	t2 := task.NewBuilder().DestroyK8SESCluster(&m.localExe, gOpt).DestroyEKSCluster(&m.localExe, "es", gOpt).BuildAsStep("  - Destroying ES cluster")
	destroyTasks = append(destroyTasks, t2)

	t3 := task.NewBuilder().DestroyNAT(&m.localExe, "kafka").DestroyEC2Nodes(&m.localExe, "kafka").BuildAsStep(fmt.Sprintf("  - Destroying kafka nodes cluster %s ", name))
	destroyTasks = append(destroyTasks, t3)

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
func (m *Manager) ListTiDB2Kafka2ESCluster(clusterName, clusterType string, opt DeployOptions) error {

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
	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	// workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	// if err != nil {
	// 	return err
	// }

	/* ********** ********** 004 Prepare insert query to /opt/kafka/query.sql **********/
	strInsQuery := fmt.Sprintf("insert into test.test01(col02) values(1)")
	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("echo \\\"%s\\\" > /tmp/query.sql", strInsQuery), true); err != nil {
		return err
	}

	if _, _, err := m.wsExe.Execute(ctx, "mv /tmp/query.sql /opt/kafka/", true); err != nil {
		return err
	}

	if _, _, err := m.wsExe.Execute(ctx, "chmod 777 /opt/kafka/query.sql", true); err != nil {
		return err
	}

	// 006.01 Reset test01 test table
	commands := []string{
		"drop table if exists test01",
		fmt.Sprintf("create table test01(col01 bigint auto_random primary key, col02 int)"),
	}

	for _, command := range commands {
		_, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query test '%s'", command), false, 1*time.Hour)
		if err != nil {
			return err
		}
	}

	timer.Take("03. Table creation in the TiDB")

	/* ********** ********** 007 Prepare kafka related objects  **********/

	// 007.02 Script create topic for multiple partition in advanced.
	_, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %s %d", "test_test01", perfOpt.Partitions), false, 1*time.Hour)
	if err != nil {
		return err
	}

	timer.Take("04. Create kafka topic in advanced for multiple parations per table - /opt/kafka/source.toml")

	/* ********** ********** 008 Extract server info(ticdc/broker/schema registry/ connector)   **********/
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
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

	stdout, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --server http://%s:9300 2>/dev/null", cdcIP), false)
	if err != nil {
		return err
	}

	/* ********** ********** 009 Prepare TiCDC source changefeed   **********/
	// 008.01 TiCDC source config file
	if err = m.wsExe.TransferTemplate(ctx, "templates/config/ticdc.source.toml.tpl", "/opt/kafka/source.toml", "0644", []string{}, true, 0); err != nil {
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
		if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed create --server http://%s:9300 --changefeed-id='%s' --sink-uri='kafka://%s:9092/%s?protocol=avro&replication-factor=3' --schema-registry=http://%s:8081 --config %s", cdcIP, "kafka-avro", brokerIP, "topic-name", schemaRegistryIP, "/opt/kafka/source.toml"), false); err != nil {
			return err
		}
	}

	esHost, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("kubectl get service nginx-ingress-controller-ingress-nginx-controller-internal -o custom-columns=:.status.loadBalancer.ingress[0].hostname | awk '{printf $1}'"), false)
	if err != nil {
		return err
	}

	timer.Take("06. Create if not exists changefeed of TiCDC")

	esConfig := make(map[string]string)
	esConfig["ES_IP"] = string(esHost)
	esConfig["ES_User"] = "elastic"
	esConfig["ES_Password"] = "1234Abcd"
	esConfig["SchemaRegistry"] = schemaRegistryIP
	esConfig["TopicName"] = "test_test01"
	if err = m.wsExe.TransferTemplate(ctx, "templates/config/tidb2kafka2es/kafka.sink.json", "/opt/kafka/es.sink.json", "0644", esConfig, true, 0); err != nil {
		return err
	}

	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/es.sink.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
		return err
	}
	timer.Take("07. Create the kafka sink to postgres")

	timer.Print()

	return nil
}

func (m *Manager) PerfTiDB2Kafka2ES(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	// 01. Get the workstation executor
	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	// workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	// if err != nil {
	// 	return err
	// }

	// 02. Get the TiDB connection info
	if err := m.wsExe.Transfer(ctx, "/opt/tidb-db-info.yml", "/tmp/tidb-db-info.yml", true, 1024); err != nil {
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

	_, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query mysql '%s'", "drop database if exists mysqlslap"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	// 03. Prepare the query to insert data into TiDB
	_, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "truncate table test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	stdout, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "truncate table  test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}

	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("mysqlslap --no-defaults -h %s -P %d --user=%s --query=/opt/kafka/query.sql --concurrency=%d --iterations=%d --number-of-queries=%d --create='drop database if exists test02;create schema test02' --no-drop", tidbConnectInfo.TiDBHost, tidbConnectInfo.TiDBPort, tidbConnectInfo.TiDBUser, 10, 1, perfOpt.NumOfRecords), true, 1*time.Hour)
	if err != nil {
		return err
	}
	timer.Take("mysqlslap running")

	// 04. Wait the data sync to postgres
	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	tidbCnt := strings.Trim(string(stdout), "\n")

	for cnt := 0; cnt < 20; cnt++ {
		time.Sleep(10 * time.Second)
		stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
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
	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	cnt := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select count(*)/(EXTRACT(EPOCH FROM max(tidb_timestamp) - min(tidb_timestamp))::int + 1) from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	tidbQPS := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select EXTRACT(EPOCH FROM (sum(pg_timestamp - tidb_timestamp)/count(*)))::int  from test01"), false, 1*time.Hour)
	if err != nil {
		return err
	}
	latency := strings.Trim(strings.Trim(string(stdout), "\n"), " ")

	stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_pg_query %s '%s'", "test", "select (count(*)/EXTRACT(EPOCH FROM max(pg_timestamp) - min(tidb_timestamp)))::int as min_pg from test01"), false, 1*time.Hour)
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
	// sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	// if err != nil {
	// 	return err
	// }

	// workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, clusterType, gOpt.SSHUser, gOpt.IdentityFile)
	// if err != nil {
	// 	return err
	// }

	// Get server info
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	var tableECs [][]string
	t1 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
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
	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("curl -X DELETE http://%s:8083/connectors/%s", connectorIP, "ESSINK"), false); err != nil {
		return err
	}

	// Remove the changefeed
	stdout, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --server http://%s:9300 2>/dev/null", cdcIP), false)
	if err != nil {
		return err
	}
	changeFeedExist, err := changeFeedExist(stdout, "kafka-avro")
	if err != nil {
		return err
	}
	if changeFeedExist == true {
		if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed remove --server http://%s:9300 --changefeed-id='%s'", cdcIP, "kafka-avro"), false); err != nil {
			return err
		}
	}

	// Remove the topic

	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, schemaRegistryIP, "sudo systemctl stop confluent-schema-registry"), false, 600*time.Second); err != nil {
		return err
	}

	for _, topicName := range []string{"topic-name", "test_test01", "_schemas"} {
		if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka-util.sh remove-topic %s", topicName), false); err != nil {
			logger.OutputDebugLog(err.Error())
			//return err
		}
	}
	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, schemaRegistryIP, "sudo systemctl start confluent-schema-registry"), false, 600*time.Second); err != nil {
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
