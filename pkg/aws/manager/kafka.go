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

// DeployOptions contains the options for scale out.
type KafkaDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) KafkaDeploy(
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

	clusterType := "ohmytiup-kafka"

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
		envInitTasks = append(envInitTasks, t1)
	}

	// Setup the kafka cluster
	t2 := task.NewBuilder().CreateKafkaCluster(&sexecutor, "kafka", base.AwsKafkaTopoConfigs, &clusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	envInitTasks = append(envInitTasks, t2)

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
		CreateRouteTgw(&sexecutor, "workstation", []string{"kafka"}).
		DeployKafka(&sexecutor, base.AwsWSConfigs, "kafka", &workstationInfo).
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
func (m *Manager) DestroyKafkaCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "ohmytiup-kafka"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		// DestroyVpcPeering(&sexecutor).
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

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListKafkaCluster(clusterName string, opt DeployOptions) error {
	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "ohmytiup-kafka")

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

// Scale a cluster.
func (m *Manager) KafkaScale(
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
	clusterType := "ohmytiup-kafka"

	// var workstationInfo, clusterInfo task.ClusterInfo
	var workstationInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)

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

type KafkaPerfOpt struct {
	// Arguments for consumer/producer and E2E
	Partitions    int // The number of partitions
	NumOfRecords  int // The number of records to be tested
	BytesOfRecord int // The Bytes per record to be tested

	// End 2 End performance test
	ProducerAcks string

	// DataType
	DataTypeDtr []string
}

func (m *Manager) PerfKafkaPC(clusterName string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	ctx := ctxt.New(context.Background(), 1)

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, "ohmytiup-kafka", gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	topicName := "test-topic"
	// Create the partition
	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %d", topicName, perfOpt.Partitions), false, 1*time.Hour)
	if err != nil {
		return err
	}

	// Producer performance measurement
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.producer.perf.sh %d %d", topicName, perfOpt.NumOfRecords, perfOpt.BytesOfRecord), false, 1*time.Hour)
	if err != nil {
		return err
	}
	output := strings.Split(string(stdout), "\n")
	rows := strings.Split(output[len(output)-2], ",")

	tableMetrics := [][]string{{"records sent", "records/sec", "avg latency", "max latency", "50th", "95th", "99th", "99.9th"}}
	var metrics []string
	for idx, metric := range rows {
		value := strings.Split(string(strings.Trim(metric, " ")), " ")
		if idx > 1 {
			metrics = append(metrics, value[0]+" "+value[1])
		} else {
			metrics = append(metrics, value[0])
		}

	}

	// Consumer performance measurment
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.consumer.perf.sh %d", topicName, perfOpt.NumOfRecords), false, 1*time.Hour)
	if err != nil {
		return err
	}

	var tableConsumerMetrics [][]string
	var rowHeader, rowMetric []string

	arrConsumer := strings.Split(string(stdout), "\n")
	for _, metric := range arrConsumer {
		arrMetrics := strings.Split(strings.Trim(string(metric), " "), ":")
		if len(arrMetrics) < 2 {
			continue
		}
		rowHeader = append(rowHeader, arrMetrics[0])
		rowMetric = append(rowMetric, strings.Trim(strings.Join(arrMetrics[1:], ":"), " "))
	}

	tableConsumerMetrics = append(tableConsumerMetrics, rowHeader)
	tableConsumerMetrics = append(tableConsumerMetrics, rowMetric)

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("\nPerformance test:      %s\n", titleFont.Sprint("Producer"))

	tableMetrics = append(tableMetrics, metrics)
	tui.PrintTable(tableMetrics, true)

	fmt.Printf("\n\nPerformance test:      %s\n", titleFont.Sprint("Consumer"))
	tui.PrintTable(tableConsumerMetrics, true)

	return nil
}

func (m *Manager) PerfKafkaE2E(clusterName string, perfOpt KafkaPerfOpt, gOpt operator.Options) error {

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	ctx := ctxt.New(context.Background(), 1)

	workstation, err := task.GetWSExecutor(sexecutor, ctx, clusterName, "ohmytiup-kafka", gOpt.SSHUser, gOpt.IdentityFile)
	if err != nil {
		return err
	}

	// Create the partition
	stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.create.topic.sh %d", perfOpt.Partitions), false, 1*time.Hour)
	if err != nil {
		return err
	}

	// Producer performance measurement
	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/opt/kafka/perf/kafka.e2e.perf.sh %d %s %d", perfOpt.NumOfRecords, perfOpt.ProducerAcks, perfOpt.BytesOfRecord), false, 2*time.Hour)
	if err != nil {
		return err
	}
	output := strings.Split(string(stdout), "\n")

	rows := strings.Split(output[len(output)-4], ":")

	var tableConsumerMetrics [][]string
	var rowHeader, rowMetric []string
	rowHeader = append(rowHeader, rows[0])
	rowMetric = append(rowMetric, rows[1])

	rows = strings.Split(output[len(output)-2], ":")
	rows = strings.Split(rows[1], ",")

	for _, item := range rows {
		item := strings.Split(item, "=")
		rowHeader = append(rowHeader, item[0])
		rowMetric = append(rowMetric, item[1])
	}

	tableConsumerMetrics = append(tableConsumerMetrics, rowHeader)
	tableConsumerMetrics = append(tableConsumerMetrics, rowMetric)

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("\nPerformance test:      %s\n", titleFont.Sprint("End 2 End"))

	tui.PrintTable(tableConsumerMetrics, true)

	return nil
}
