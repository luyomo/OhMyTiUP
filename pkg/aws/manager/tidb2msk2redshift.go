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

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"

	perrs "github.com/pingcap/errors"
)

// Deploy a cluster.
func (m *Manager) TiDB2Msk2RedshiftDeploy(
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

	// -- redshift ----------------------------------------------------------->
	// -- workstation cluster   | --> routes --> | --> TiDB instance deployment
	// -- tidb cluster          |                | --> kafka insance deployment
	// -- kafka cluster         |

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, false, false); err != nil {
		return err
	}

	var workstationInfo, clusterInfo, mskClusterInfo, redshiftClusterInfo task.ClusterInfo

	var mainTask []*task.StepDisplay // tasks which are used to initialize environment

	// Task: Redshift deployment.
	redshiftTask := task.NewBuilder().
		CreateRedshiftCluster(&m.localExe, "redshift", base.AwsRedshiftTopoConfigs, &redshiftClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing redshift servers"))
	mainTask = append(mainTask, redshiftTask)

	glueTask := task.NewBuilder().
		CreateGlueSchemaRegistryCluster(&m.localExe).
		BuildAsStep(fmt.Sprintf("  - Preparing glue schema registry ... ... "))
	mainTask = append(mainTask, glueTask)

	// Parallel task to create workstation, tidb, kafka resources and transit gateway.
	var task001 []*task.StepDisplay // tasks which are used to initialize environment

	t1 := task.NewBuilder().
		CreateWorkstationCluster(&m.localExe, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	task001 = append(task001, t1)

	//Setup the kafka cluster
	t2 := task.NewBuilder().CreateMSKCluster(&m.localExe, "msk", base.AwsMSKTopoConfigs, &mskClusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing kafka servers"))
	task001 = append(task001, t2)

	t3 := task.NewBuilder().CreateTiDBCluster(&m.localExe, "tidb", base.AwsTopoConfigs, &clusterInfo).BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	task001 = append(task001, t3)

	t4 := task.NewBuilder().CreateTransitGateway(&m.localExe).BuildAsStep(fmt.Sprintf("  - Preparing the transit gateway"))
	task001 = append(task001, t4)

	// Parallel task to create tidb and kafka instances
	var task002 []*task.StepDisplay // tasks which are used to initialize environment

	t23 := task.NewBuilder().
		DeployTiDB(&m.localExe, "tidb", base.AwsWSConfigs, &workstationInfo).
		DeployTiDBInstance(&m.localExe, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo).
		CreateTiCDCGlue(&m.wsExe).
		BuildAsStep(fmt.Sprintf("  - Deploying tidb instance ... "))
	task002 = append(task002, t23)

	t24 := task.NewBuilder().
		DeployKafka(&m.localExe, base.AwsWSConfigs, "msk", &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Deploying kafka instance ... "))
	task002 = append(task002, t24)

	t25 := task.NewBuilder().
		DeployRedshiftInstance(&m.localExe, base.AwsWSConfigs, base.AwsRedshiftTopoConfigs, &m.wsExe).
		BuildAsStep(fmt.Sprintf("  - Deploying redshift resource ... "))
	task002 = append(task002, t25)

	t26 := task.NewBuilder().
		CreateMSKConnectPlugin(&m.localExe, &m.wsExe, base.AwsMSKConnectPluginTopoConfigs).
		BuildAsStep(fmt.Sprintf("  - Preparing msk connect plugin ... ... "))
	task002 = append(task002, t26)

	// The es might be lag behind the tidb/kafka cluster
	// Cluster generation -> transit gateway setup -> instance deployment
	paraTask001 := task.NewBuilder().
		ParallelStep("+ Deploying all the sub components for kafka solution service", false, task001...).
		CreateRouteTgw(&m.localExe, "workstation", []string{"tidb", "redshift", "msk"}).
		CreateRouteTgw(&m.localExe, "msk", []string{"tidb", "redshift"}).
		RunCommonWS(&m.wsExe, &[]string{"git", "maven", "zip"}).
		ParallelStep("+ Deploying all the sub components for kafka solution service", false, task002...).
		BuildAsStep("Parallel Main step")

	// Combine the Redshift deployment and other resources

	mainTask = append(mainTask, paraTask001)

	mainBuilder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for kafka solution service", false, mainTask...).Build()

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
func (m *Manager) DestroyTiDB2Msk2RedshiftCluster(name, clusterType string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {

	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := m.makeExeContext(ctx, nil, &gOpt, false, false); err != nil {
		return err
	}

	// gOpt.SSHUser, gOpt.IdentityFile

	var destroyTasks []*task.StepDisplay

	t1 := task.NewBuilder().DestroyTransitGateways(&m.localExe).BuildAsStep("  - Removing transit gateway")
	destroyTasks = append(destroyTasks, t1)

	t2 := task.NewBuilder().DestroyRedshiftCluster(&m.localExe, "redshift").BuildAsStep("  - Destroying Redshift cluster")
	destroyTasks = append(destroyTasks, t2)

	t5 := task.NewBuilder().DestroyMSKCluster(&m.localExe, "msk").BuildAsStep("  - Destroying MSK cluster")
	destroyTasks = append(destroyTasks, t5)

	t3 := task.NewBuilder().DestroyNAT(&m.localExe, "msk").DestroyEC2Nodes(&m.localExe, "kafka").BuildAsStep(fmt.Sprintf("  - Destroying kafka nodes cluster %s ", name))
	destroyTasks = append(destroyTasks, t3)

	t4 := task.NewBuilder().DestroyNAT(&m.localExe, "tidb").DestroyEC2Nodes(&m.localExe, "tidb").BuildAsStep(fmt.Sprintf("  - Destroying  tidb cluster %s ", name))
	destroyTasks = append(destroyTasks, t4)

	t6 := task.NewBuilder().DestroyGlueSchemaRegistry(&m.localExe).BuildAsStep(fmt.Sprintf("  - Destroying glue schema registry %s ", name))
	destroyTasks = append(destroyTasks, t6)

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
func (m *Manager) ListTiDB2Msk2RedshiftCluster(clusterName, clusterType string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, nil, false, false); err != nil {
		return err
	}

	// 001. VPC listing
	tableVPC := [][]string{{"Component Name", "VPC ID", "CIDR", "Status"}}
	t1 := task.NewBuilder().ListVpc(&m.localExe, &tableVPC).BuildAsStep(fmt.Sprintf("  - Listing VPC"))
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
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// 008. NLB
	var nlb elbtypes.LoadBalancer
	t8 := task.NewBuilder().ListNLB(&m.localExe, "tidb", &nlb).BuildAsStep(fmt.Sprintf("  - Listing Load Balancer "))
	listTasks = append(listTasks, t8)

	// 009. Redshift
	var redshiftDBInfos task.RedshiftDBInfos
	t9 := task.NewBuilder().ListRedshiftCluster(&m.localExe, &redshiftDBInfos).BuildAsStep(fmt.Sprintf("  - Listing Redshift"))
	listTasks = append(listTasks, t9)

	// 010. MSK
	var mskDBInfos task.MSKInfos
	t10 := task.NewBuilder().ListMSKCluster(&m.localExe, &mskDBInfos).BuildAsStep(fmt.Sprintf("  - Listing MSK"))
	listTasks = append(listTasks, t10)

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

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("REDSHIFT"))
	tui.PrintTable(*redshiftDBInfos.ToPrintTable(), true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("MSK"))
	tui.PrintTable(*mskDBInfos.ToPrintTable(), true)

	return nil
}

/*
****************************************************************************

Parameters:

	perfOpt
	  -> DataTypeDtr: ["int", "varchar"]
*/
func (m *Manager) PerfPrepareTiDB2MSK2Redshift(clusterName, clusterType string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {
	/* ********** ********** 001. Read the column mapping file to struct
		   ColumnMapping.yml:
		   MapTiDB2PG:
		   - TiDB:
		       DataType: BOOL
		       Def: t_bool BOOL
		     PG:
		       DataType: BOOL
		       Def: t_bool BOOL
		    Value: true
		    ... ...
	           - TiDB:
	               DataType: SET
	               Def: t_set SET('"'"'a'"'"','"'"'b'"'"','"'"'c'"'"')
	             PG:
	               DataType: ENUM
	               Def: t_set t_enum_test[]
	             Query:
	               - create type t_enum_test as enum  ('"'"'a'"'"','"'"'b'"'"','"'"'c'"'"')
	*/
	/*
			   1. Read map from config file           -> BasePerf
			   2. Dialect ddl generator
			      2.1 source database ddl preparation
		      	      2.2 target database ddl preparation
			   3. Dialect insert sql
			   4. changefeed creation
			   5. sink preparation
	*/

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

	if err := m.makeExeContext(ctx, nil, &gOpt, true, false); err != nil {
		return err
	}

	// Get the redshift data
	redshiftDBInfo, err := m.workstation.GetRedshiftDBInfo()
	if err != nil {
		return err
	}
	// var redshiftDBInfos []ws.RedshiftDBInfo
	// err := m.workstation.ParseYamlConfig("/opt/redshift.dbinfo.yaml", &redshiftDBInfos)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The config is <%#v> \n\n\n", redshiftDBInfos)

	// Get AWS MSK data
	var mskDBInfos task.MSKInfos
	t1 := task.NewBuilder().ListMSKCluster(&m.localExe, &mskDBInfos).Build()
	if err := t1.Execute(ctxt.New(ctx, 10)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	mskEndpoints, err := mskDBInfos.GetFirstEndpoint()
	if err != nil {
		return err
	}

	region, err := awsutils.GetDefaultRegion()
	if err != nil {
		return err
	}

	t2 := task.NewBuilder().
		// CreatePerfTables(&m.wsExe, "embed/templates/config/tidb2kafka2redshift/ColumnMapping.yml", strings.Split("BOOL,TINYINT,SMALLINT", ",")).
		// CreateChangefeed(&m.wsExe, mskEndpoints).
		CreateWorkerConfiguration().
		CreateServiceIamPolicy().
		CreateServiceIamRole().
		CreateMskConnect(&m.wsExe, &task.CreateMskConnectInput{
			RedshiftDBInfo:     redshiftDBInfo,
			MskEndpoints:       mskEndpoints,
			GlueSchemaRegistry: "tidb2redshift",
			Region:             *region,
			TopicName:          "test_test02",
			TableName:          "test02"}).
		Build()
	if err := t2.Execute(ctxt.New(ctx, 10)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	// BuildAsStep(fmt.Sprintf("  - Creating perf tables"))

	// perfTasks = append(perfTasks, t1)

	// // builder := task.NewBuilder().ParallelStep("+ Destroying all the componets", false, perfTasks...)

	// t := builder.Build()

	// if err := t1.Execute(context.Background()); err != nil {
	// 	if errorx.Cast(err) != nil {
	// 		// FIXME: Map possible task errors and give suggestions.
	// 		return err
	// 	}
	// 	return err
	// }

	return nil

	// mapFile, err := ioutil.ReadFile("embed/templates/config/tidb2kafka2redshift/ColumnMapping.yml")
	// if err != nil {
	// 	return err
	// }

	// var mapTiDB2PG MapTiDB2PG
	// err = yaml.Unmarshal(mapFile, &mapTiDB2PG)
	// if err != nil {
	// 	return err
	// }

	// /* ********** ********** 002. Prepare columns defintion to be executed into TiDB and redshift ********** ********** */
	// var arrTiDBTblDataDef []string // Array to keep tidb column definition. ex: ["pk_col BIGINT PRIMARY KEY AUTO_RANDOM", ... , "tidb_timestamp timestamp default current_timestamp"]
	// var arrPGTblDataDef []string   // Array to keep redshift column definition. ex: ["pk_col bigint PRIMARY KEY", ... ... "tidb_timestamp timestamp", "pg_timestamp timestamp default current_timestamp"]
	// var arrCols []string           // Array to keep all column names. ex: ["t_bool"]
	// var arrData []string           // Array to keep data to be inserted. ex: ["true"]
	// var pgPreQueries []string      // Array of queries to be executed in PG. ex: ["create type t_enum_test ..."]

	// /* 002.01 Prepare primary key column definition */
	// arrTiDBTblDataDef = append(arrTiDBTblDataDef, "pk_col BIGINT PRIMARY KEY AUTO_RANDOM")
	// arrPGTblDataDef = append(arrPGTblDataDef, "pk_col bigint PRIMARY KEY")

	// /* 002.02 Prepare column definition body */
	// for _, _dataType := range perfOpt.DataTypeDtr {
	// 	for _, _mapItem := range mapTiDB2PG.TiDB2PG {
	// 		if _dataType == _mapItem.TiDB.DataType {
	// 			arrTiDBTblDataDef = append(arrTiDBTblDataDef, _mapItem.TiDB.Def)
	// 			arrPGTblDataDef = append(arrPGTblDataDef, _mapItem.PG.Def)
	// 			arrCols = append(arrCols, strings.Split(_mapItem.TiDB.Def, " ")[0])
	// 			arrData = append(arrData, strings.Replace(strings.Replace(_mapItem.Value, "<<<<", "'", 1), ">>>>", "'", 1))
	// 			pgPreQueries = append(pgPreQueries, _mapItem.PG.Queries...)
	// 		}
	// 	}
	// }

	// /* 002.03 Prepare tail columns for both TiDB and redshift tables.*/
	// arrTiDBTblDataDef = append(arrTiDBTblDataDef, "tidb_timestamp timestamp default current_timestamp")
	// arrPGTblDataDef = append(arrPGTblDataDef, "tidb_timestamp timestamp")
	// arrPGTblDataDef = append(arrPGTblDataDef, "pg_timestamp timestamp default current_timestamp")

	// /* ********** ********** 003. Prepare execution context **********/
	// ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	// ctx = context.WithValue(ctx, "clusterType", clusterType)

	// var timer awsutils.ExecutionTimer
	// timer.Initialize([]string{"Step", "Duration(s)"})

	// if err := m.makeExeContext(ctx, nil, &gOpt, true, true); err != nil {
	// 	return err
	// }

	// /* ********** ********** 004 Prepare insert query to /opt/kafka/query.sql **********/
	// strInsQuery := fmt.Sprintf("insert into test.test01(%s) values(%s)", strings.Join(arrCols, ","), strings.Join(arrData, ","))
	// if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("echo \\\"%s\\\" > /tmp/query.sql", strInsQuery), true); err != nil {
	// 	return err
	// }

	// if _, _, err := m.wsExe.Execute(ctx, "mv /tmp/query.sql /opt/kafka/", true); err != nil {
	// 	return err
	// }

	// if _, _, err := m.wsExe.Execute(ctx, "chmod 777 /opt/kafka/query.sql", true); err != nil {
	// 	return err
	// }

	// /* ********** ********** 005 Prepare redshift objects  **********/
	// // 005.01 Reset test database if exists

	// stdout, _, err := m.wsExe.Execute(ctx, fmt.Sprintf(`/opt/scripts/run_redshift_query dev "%s"`, fmt.Sprintf("select count(*) from pg_database where datname = '%s'", "test")), false, 1*time.Hour)
	// if err != nil {
	// 	return err
	// }
	// cntTable := strings.ReplaceAll(strings.ReplaceAll(string(stdout), " ", ""), "\n", "")

	// if cntTable > "0" {
	// 	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_redshift_query dev '%s'", "drop database test"), false, 1*time.Hour); err != nil {
	// 		return err
	// 	}
	// }

	// if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_redshift_query dev '%s'", "create database test"), false, 1*time.Hour); err != nil {
	// 	return err
	// }
	// timer.Take("01. Redshift DB creation")

	// // 005.02 Create redshift objects for test. Like enum
	// for _, query := range pgPreQueries {
	// 	_, stderr, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_redshift_query test '%s'", query), false, 1*time.Hour)
	// 	if err != nil {
	// 		logger.OutputDebugLog(string(stderr))
	// 		return err
	// 	}
	// }

	// // 005. 03 Create test table
	// commands := []string{
	// 	fmt.Sprintf("create table test01(%s)", strings.Join(arrPGTblDataDef, ",")),
	// }

	// for _, command := range commands {
	// 	_, stderr, err := m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_redshift_query test '%s'", command), false, 1*time.Hour)
	// 	if err != nil {
	// 		logger.OutputDebugLog(string(stderr))
	// 		return err
	// 	}
	// }
	// timer.Take("02. Table Creation in the Redshift")

	// /* ********** ********** 006 Prepare redshift objects  **********/
	// // 006.01 Reset test01 test table
	// commands = []string{
	// 	"drop table if exists test01",
	// 	fmt.Sprintf("create table test01(%s)", strings.Join(arrTiDBTblDataDef, ",")),
	// }

	// for _, command := range commands {
	// 	if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query test '%s'", command), false, 1*time.Hour); err != nil {
	// 		return err
	// 	}
	// }

	// timer.Take("03. Table creation in the TiDB")

	// /* ********** ********** 007 Prepare kafka related objects  **********/

	// // 007.02 Script create topic for multiple partition in advanced.
	// if _, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %s %d", "test_test01", perfOpt.Partitions), false, 1*time.Hour); err != nil {
	// 	return err
	// }

	// timer.Take("04. Create kafka topic in advanced for multiple parations per table - /opt/kafka/source.toml")

	// /* ********** ********** 008 Extract server info(ticdc/broker/schema registry/ connector)   **********/
	// var listTasks []*task.StepDisplay // tasks which are used to initialize environment
	// var tableECs [][]string
	// t1 := task.NewBuilder().ListEC(&m.localExe, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	// listTasks = append(listTasks, t1)

	// builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	// t := builder.Build()

	// if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
	// 	return err
	// }

	// var cdcIP, schemaRegistryIP, brokerIP, connectorIP string
	// for _, row := range tableECs {
	// 	if row[0] == "ticdc" {
	// 		cdcIP = row[5]
	// 	}
	// 	if row[0] == "broker" {
	// 		brokerIP = row[5]
	// 	}
	// 	if row[0] == "schemaRegistry" {
	// 		schemaRegistryIP = row[5]
	// 	}
	// 	if row[0] == "connector" {
	// 		connectorIP = row[5]
	// 	}
	// }
	// timer.Take("05. Get required info - pd/broker/schemaRegistry/connector")

	// stdout, _, err = m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed list --server http://%s:8300 2>/dev/null", cdcIP), false)
	// if err != nil {
	// 	return err
	// }

	// /* ********** ********** 009 Prepare TiCDC source changefeed   **********/
	// // 008.01 TiCDC source config file
	// if err = m.wsExe.TransferTemplate(ctx, "templates/config/ticdc.source.toml.tpl", "/opt/kafka/source.toml", "0644", []string{}, true, 0); err != nil {
	// 	return err
	// }

	// // 008.02 Extract changefeed status
	// type ChangeFeed struct {
	// 	Id string `json:"id"`
	// 	// Summary ChangeFeedSummary `json:"summary"`
	// 	Summary struct {
	// 		State      string `json:"state"`
	// 		Tso        int    `json:"tso"`
	// 		Checkpoint string `json:"checkpoint"`
	// 		Error      string `json:"error"`
	// 	} `json:"summary"`
	// }
	// var changeFeeds []ChangeFeed

	// if err = yaml.Unmarshal(stdout, &changeFeeds); err != nil {
	// 	return err
	// }

	// changeFeedHasExisted := false
	// for _, changeFeed := range changeFeeds {
	// 	if changeFeed.Id == "kafka-avro" {
	// 		changeFeedHasExisted = true
	// 	}
	// }

	// // 009.03 Create changefeed if it does not exists
	// if changeFeedHasExisted == false {
	// 	if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup cdc cli changefeed create --server http://%s:8300 --changefeed-id='%s' --sink-uri='kafka://%s:9092/%s?protocol=avro' --schema-registry=http://%s:8081 --config %s", cdcIP, "kafka-avro", brokerIP, "topic-name", schemaRegistryIP, "/opt/kafka/source.toml"), false); err != nil {
	// 		return err
	// 	}
	// }

	// timer.Take("06. Create if not exists changefeed of TiCDC")

	// // 009.04 Fetch TiDB connection infro from /opt/db-info.yml
	// if err = m.wsExe.Transfer(ctx, "/opt/redshift.dbinfo.yaml", "/tmp/redshift.dbinfo.yaml", true, 1024); err != nil {
	// 	return err
	// }

	// yfile, err := ioutil.ReadFile("/tmp/redshift.dbinfo.yaml")
	// if err != nil {
	// 	return err
	// }

	// var redshiftDBInfos []task.RedshiftDBInfo
	// err = yaml.Unmarshal(yfile, &redshiftDBInfos)
	// if err != nil {
	// 	return err
	// }
	// redshiftConfig := make(map[string]string)

	// if len(redshiftDBInfos) == 1 {
	// 	redshiftConfig["RedshiftHost"] = redshiftDBInfos[0].Host
	// 	redshiftConfig["RedshiftPort"] = fmt.Sprintf("%d", redshiftDBInfos[0].Port)
	// 	redshiftConfig["RedshiftUser"] = redshiftDBInfos[0].UserName
	// 	redshiftConfig["RedshiftPassword"] = redshiftDBInfos[0].Password
	// } else {
	// 	errors.New("Failed to get the redshift db info")
	// }

	// redshiftConfig["SINKName"] = clusterName
	// redshiftConfig["KafkaBroker"] = fmt.Sprintf("%s:9092", brokerIP)
	// redshiftConfig["TopicName"] = "test_test01"
	// redshiftConfig["RedshiftDBName"] = "test"
	// redshiftConfig["TableName"] = "test01"

	// if err = m.wsExe.TransferTemplate(ctx, "templates/config/kafka.redshift.sink.json.tpl", "/opt/kafka/kafka.redshift.sink.json", "0644", redshiftConfig, true, 0); err != nil {
	// 	return err
	// }

	// if _, _, err := m.wsExe.Execute(ctx, fmt.Sprintf("curl -d @'/opt/kafka/kafka.redshift.sink.json' -H 'Content-Type: application/json' -X POST http://%s:8083/connectors", connectorIP), false); err != nil {
	// 	return err
	// }
	// timer.Take("07. Create the kafka sink to redshift")

	// timer.Print()

	// return nil
}
