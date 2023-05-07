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
	// "errors"
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tidbcloudapi"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	// perrs "github.com/pingcap/errors"
)

func (m *Manager) TiDBCloudDeploy(
	name, clusterType string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	base := topo.BaseTopo()

	if err := task.InitClientInstance(); err != nil {
		return err
	}

	cluster, err := tidbcloudapi.GetClusterByName(uint64(base.TiDBCloud.General.ProjectID), name)
	if err != nil {
		return err
	}
	if cluster != nil {
		m.ListTiDBCloudCluster(uint64(base.TiDBCloud.General.ProjectID), name, "ALL")

		tui.Prompt("Please confirm the project and cluster.")

		return nil
	}

	m.confirmTiDBCloudTopology(name, base.TiDBCloud)

	var envInitTasks []*task.StepDisplay // tasks which are used to initialize environment
	t1 := task.NewBuilder().CreateTiDBCloud(base.TiDBCloudConfigs).
		BuildAsStep(fmt.Sprintf("  - Preparing TiDB Cloud"))
	envInitTasks = append(envInitTasks, t1)

	builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for tidb solution service", false, envInitTasks...)

	t := builder.Build()

	ctx := context.WithValue(context.Background(), "clusterName", name)
	if err := t.Execute(ctxt.New(ctx, gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyTiDBCloudCluster(projectID uint64, name, clusterType string) error {
	err := m.findClusterToRun(projectID, name, "delete", tidbcloudapi.DeleteClusterByID)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) ResumeTiDBCloudCluster(projectID uint64, name, clusterType string) error {
	err := m.findClusterToRun(projectID, name, "resume", tidbcloudapi.ResumeClusterByID)
	if err != nil {
		return err
	}
	return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) PauseTiDBCloudCluster(projectID uint64, name, clusterType string) error {
	err := m.findClusterToRun(projectID, name, "pause", tidbcloudapi.PauseClusterByID)
	if err != nil {
		return err
	}
	return nil
}

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListTiDBCloudCluster(projectID uint64, clusterName, status string) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)

	// 001. VPC listing
	tableClusters := [][]string{{"Project", "Cluster Name", "Cluster Type", "Version", "Status", "Cloud Provider", "Region", "Create Timestamp"}}

	tableNodes := [][]string{{"Project", "Cluster Name", "Component Type", "Node Size", "Count", "Storage Size"}}
	t1 := task.NewBuilder().ListTiDBCloud(projectID, status, "DEDICATED", &tableClusters, &tableNodes).BuildAsStep(fmt.Sprintf("  - Listing TiDB Cloud"))
	listTasks = append(listTasks, t1)

	// *********************************************************************
	builder := task.NewBuilder().ParallelStep("+ Listing aws resources", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("TiDB Cloud"))
	fmt.Printf("Cluster Name :      %s\n\n", titleFont.Sprint(clusterName))

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Resource Type:      %s\n", cyan.Sprint("Meta Data"))
	tui.PrintTable(tableClusters, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Node Info"))
	tui.PrintTable(tableNodes, true)

	return nil
}

// Scale a cluster.
func (m *Manager) TiDBCloudScale(
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
	// clusterType := "ohmytiup-tidbcloud"

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo, &m.wsExe, &gOpt).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	ctx = context.WithValue(ctx, "tagOwner", gOpt.TagOwner)
	ctx = context.WithValue(ctx, "tagProject", gOpt.TagProject)

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

func (m *Manager) confirmTiDBCloudTopology(name string, specTiDBCloud *spec.TiDBCloud) error {
	log.Infof("Please confirm your TiDB Cloud topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Project ID:      %s\n", cyan.Sprint(strconv.FormatInt((*specTiDBCloud).General.ProjectID, 10)))
	fmt.Printf("Region    :      %s\n", cyan.Sprint((*specTiDBCloud).General.Region))
	fmt.Printf("Port      :      %s\n", cyan.Sprint(strconv.FormatInt(int64((*specTiDBCloud).General.Port), 10)))

	clusterTable := [][]string{
		// Header
		{"Component", "# of nodes", "Node Size", "Storage Size"},
	}

	clusterTable = append(clusterTable, []string{"TiDB", strconv.FormatInt(int64((*specTiDBCloud).TiDB.Count), 10), (*specTiDBCloud).TiDB.NodeSize, "-"})
	clusterTable = append(clusterTable, []string{"TiKV", strconv.FormatInt(int64((*specTiDBCloud).TiKV.Count), 10), (*specTiDBCloud).TiKV.NodeSize, strconv.FormatInt(int64((*specTiDBCloud).TiKV.Storage), 10)})
	clusterTable = append(clusterTable, []string{"TiFlash", strconv.FormatInt(int64((*specTiDBCloud).TiFlash.Count), 10), (*specTiDBCloud).TiFlash.NodeSize, "-"})

	tui.PrintTable(clusterTable, true)

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")

}

// ---------- ---------- ---------- ---------- ---------- ----------
func (m *Manager) findClusterToRun(projectID uint64, clusterName, operation string, funcOp func(uint64, uint64) error) error {
	if err := task.InitClientInstance(); err != nil {
		return err
	}

	var cluster *tidbcloudapi.Cluster

	// 01. Check whether the project id is valid.
	isValid, err := tidbcloudapi.IsValidProjectID(projectID)
	if err != nil {
		return err
	}

	if isValid == true {
		//02 Get the TiDB cluster info from project id and name
		cluster, err = tidbcloudapi.GetClusterByName(projectID, clusterName)
		if err != nil {
			return err
		}
	}

	if cluster == nil {
		// If no cluster found, try to find the cluster using cluster name without project id
		cluster, err = tidbcloudapi.GetClusterByName(0, clusterName)
		if err != nil {
			return err
		}

		if cluster != nil {
			// Ask user for confirmation. List the cluster infomation
			m.ListTiDBCloudCluster(0, clusterName, "ALL")

			ret, _ := tui.PromptForConfirmYes(fmt.Sprintf("Do you want to %s the above cluster?", operation))
			if ret == false {
				return nil
			}

		} else {
			// No cluster name found, exit
			tui.Prompt(fmt.Sprintf("No cluster <%s> found in the organization/project", clusterName))
			return nil
		}

	}

	projectID = cluster.ProjectID
	clusterID := cluster.ID

	ret, _ := tui.PromptForConfirmYes(fmt.Sprintf("Are you sure to %s the cluster: (%d:%s) ?", operation, projectID, clusterName))
	if ret == true {
		if err = funcOp(projectID, clusterID); err != nil {
			return err
		}
		m.ListTiDBCloudCluster(projectID, clusterName, "ALL")

		// tidbcloudapi.PauseClusterByName(projectID, clusterID)
	}

	return nil
}
