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
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// DeployOptions contains the options for scale out.
type OssInsightDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) OssInsightDeploy(
	clusterName string,
	numOfMillions int,
	opt OssInsightDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	if err := clusterutil.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora")

	// 1. Preparation phase
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	// 2. Fetch the workstation
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	// 3. Set the AWS environment which will be used on the workstation for S3 operation
	var env []string
	if gOpt.AWSAccessKeyID != "" {
		env = append(env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", gOpt.AWSAccessKeyID))
	}
	if gOpt.AWSSecretAccessKey != "" {
		env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", gOpt.AWSSecretAccessKey))
	}
	if gOpt.AWSRegion != "" {
		env = append(env, fmt.Sprintf("AWS_REGION=%s", gOpt.AWSRegion))
	}

	// 4. Create the workstation executor
	workstation, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: tableECs[1][6], User: gOpt.SSHUser, KeyFile: gOpt.IdentityFile}, env)
	if err != nil {
		return err
	}

	timer.Take("Preparation")

	// 5. Files deployment
	_, _, err = workstation.Execute(ctx, "rm -f /tmp/ossinsight.sql", false)
	if err != nil {
		return err
	}

	_, _, err = workstation.Execute(ctx, "mkdir -p /opt/ossinsight/sql", true)
	if err != nil {
		return err
	}

	_, _, err = workstation.Execute(ctx, "wget -P /tmp/ https://ossinsight-data.s3.amazonaws.com/ddl/ossinsight.sql", false)
	if err != nil {
		return err
	}

	_, _, err = workstation.Execute(ctx, "/opt/scripts/run_mysql_query mysql 'create database if not exists ossinsight'", false)
	if err != nil {
		return err
	}

	_, _, err = workstation.Execute(ctx, "/opt/scripts/run_mysql_from_file ossinsight /tmp/ossinsight.sql", false)
	if err != nil {
		return err
	}

	type Test struct{}
	err = workstation.TransferTemplate(ctx, "templates/scripts/import_ossinsight_data.sh", "/opt/scripts/import_ossinsight_data", "0755", Test{}, true, 0)
	if err != nil {
		return err
	}

	err = workstation.TransferTemplate(ctx, "templates/scripts/import_ossinsight_data_github_event.sh", "/opt/scripts/import_ossinsight_data_github_event", "0755", Test{}, true, 0)
	if err != nil {
		return err
	}

	err = workstation.TransferTemplate(ctx, "templates/sql/ossinsight/mouthly_ranking.sql", "/opt/ossinsight/sql/mouthly_ranking.sql", "0644", Test{}, true, 0)
	if err != nil {
		return err
	}

	err = workstation.TransferTemplate(ctx, "templates/sql/ossinsight/dynamicTrends.BarChartRace.sql", "/opt/ossinsight/sql/dynamicTrends.BarChartRace.sql", "0644", Test{}, true, 0)
	if err != nil {
		return err
	}

	err = workstation.TransferTemplate(ctx, "templates/sql/ossinsight/dynamicTrends.HistoricalRanking.sql", "/opt/ossinsight/sql/dynamicTrends.HistoricalRanking.sql", "0644", Test{}, true, 0)
	if err != nil {
		return err
	}

	err = workstation.TransferTemplate(ctx, "templates/sql/ossinsight/dynamicTrends.TopTen.sql", "/opt/ossinsight/sql/dynamicTrends.TopTen.sql", "0644", Test{}, true, 0)
	if err != nil {
		return err
	}

	timer.Take("File Upload")

	// 6. Import table data except github_event
	_, _, err = workstation.Execute(ctx, "/opt/scripts/import_ossinsight_data", false, 5*time.Hour)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	timer.Take("Data import(Except github_events)")

	// 7. Import github_event table
	_, _, err = workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/import_ossinsight_data_github_event %d %d", 1, numOfMillions), false, 5*time.Hour)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}
	timer.Take("Data import(github_events)")

	// 8. Print the execution summary
	timer.Print()

	return nil
}

func (m *Manager) OssInsightCountTables(
	clusterName string,
	opt OssInsightDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora")

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var env []string
	workstation, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: tableECs[1][6], User: gOpt.SSHUser, KeyFile: gOpt.IdentityFile}, env)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	timer.Take("Preparation phase")

	tableCount := [][]string{{"Physical Name", "Number of rows"}}

	tables := []string{"ar_internal_metadata", "blacklist_repos", "blacklist_users", "cn_orgs", "cn_repos", "collection_items", "collections", "css_framework_repos", "db_repos", "gh", "github_events", "import_logs", "js_framework_repos", "new_github_events", "nocode_repos", "osdb_repos", "programming_language_repos", "schema_migrations", "static_site_generator_repos", "users"}

	for _, table := range tables {
		stdout, _, err := workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/run_mysql_query ossinsight 'select count(*) as cnt from %s'", table), false)
		if err != nil {
			return err
		}
		tableCount = append(tableCount, []string{table, strings.Replace(string(stdout), "\n", "", -1)})
	}

	tui.PrintTable(tableCount, true)
	timer.Take("Count phase")

	timer.Print()

	return nil
}

func (m *Manager) OssInsightAdjustGithubEventsValume(
	clusterName string,
	numOfMillions int,
	opt OssInsightDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora")

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	// 1. Preparation phase
	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var env []string
	workstation, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: tableECs[1][6], User: gOpt.SSHUser, KeyFile: gOpt.IdentityFile}, env)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	// 2. Fetch the count of github_events
	timer.Take("Preparation phase")

	stdout, _, err := workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/run_mysql_query ossinsight 'select convert(count(*)/1000000, UNSIGNED) + 1  as cnt from github_events'"), false)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	// 3. Data addition phase
	timer.Take("Count phase")

	_, _, err = workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/import_ossinsight_data_github_event %s %d", strings.Replace(string(stdout), "\n", "", -1), numOfMillions), false, 5*time.Hour)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	// 4. Confirmation phase
	timer.Take("Import phase")

	tableCount := [][]string{{"Table name", "Number of Rows"}}

	stdout, _, err = workstation.Execute(ctx, "/opt/scripts/run_mysql_query ossinsight 'select count(*) as cnt from github_events'", false)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}
	tableCount = append(tableCount, []string{"github_events", strings.Replace(string(stdout), "\n", "", -1)})

	// 5. Post phase
	timer.Take("Confirm phase")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("\n%s:\n", cyan.Sprint("Number of rows:"))
	tui.PrintTable(tableCount, true)

	timer.Print()

	return nil
}

func (m *Manager) OssInsightTestCase(
	clusterName string,
	numExeTime int,
	queryType string,
	opt OssInsightDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora")

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	var env []string
	workstation, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: tableECs[1][6], User: gOpt.SSHUser, KeyFile: gOpt.IdentityFile}, env)
	if err != nil {
		fmt.Sprintf("The err is <%s> \n\n\n", err)
		return err
	}

	timer.Take("Preparation phasse")

	total := 0
	tableDuration := [][]string{{"Nth", "Execution Time(ms)"}}
	for cnt := 0; cnt < numExeTime; cnt++ {
		startTime := time.Now()
		if queryType == "count" {
			_, _, err = workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/run_mysql_query ossinsight 'select count(*)  as cnt from github_events'"), false)
			if err != nil {
				fmt.Sprintf("The err is <%s> \n\n\n", err)
				return err
			}
		} else {
			_, _, err = workstation.Execute(ctx, fmt.Sprintf("/opt/scripts/run_mysql_from_file ossinsight /opt/ossinsight/sql/%s.sql", queryType), false)
			if err != nil {
				fmt.Sprintf("The err is <%s> \n\n\n", err)
				return err
			}
		}

		diff := (time.Now()).Sub(startTime)
		tableDuration = append(tableDuration, []string{strconv.Itoa(cnt + 1), strconv.Itoa(int(diff.Milliseconds()))})
		total += int(diff.Milliseconds())
		// fmt.Printf("Time taken is <%#v>", int(diff.Milliseconds()))
	}

	tableDuration = append(tableDuration, []string{"average", strconv.Itoa(int(total / numExeTime))})
	tui.PrintTable(tableDuration, true)

	timer.Take("Execution phase")

	timer.Print()

	return nil
}

func (m *Manager) OssInsightHistoricalTest(
	clusterName string,
	opt OssInsightDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	// Sever Spec
	// | Memory    | CPU    |   Query 01  | Number of rows of github  | Number of Times | Average Time  | Dates | Times |
	// -----------------------------------------------------------------------------------------------------------------

	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListOssInsight(clusterName string, opt DeployOptions) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-aurora")

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	var timer awsutils.ExecutionTimer
	timer.Initialize([]string{"Step", "Duration(s)"})

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
	tableECs := [][]string{{"Component Name", "Component Cluster", "State", "Instance ID", "Instance Type", "Preivate IP", "Public IP", "Image ID"}}
	t7 := task.NewBuilder().ListEC(&sexecutor, &tableECs).BuildAsStep(fmt.Sprintf("  - Listing EC2"))
	listTasks = append(listTasks, t7)

	// 008. NLB
	var nlb elbtypes.LoadBalancer
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

	timer.Take("Take Info Phase")

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("ohmytiup-aurora"))
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

	timer.Print()

	return nil
}

func (m *Manager) DestroyOssInsight(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "ohmytiup-aurora"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		// DestroyVpcPeering(&sexecutor).
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
