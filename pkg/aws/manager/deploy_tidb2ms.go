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
	//	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	//	"github.com/luyomo/OhMyTiUP/pkg/crypto"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	//	"go.uber.org/zap"
	"os"
	//"strings"
)

// DeployOptions contains the options for scale out.
type TiDB2MSDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) TiDB2MSDeploy(
	name string,
	topoFile string,
	opt TiDB2MSDeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) error {
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
	clusterType := "ohmytiup-tidb2ms"

	var workstationInfo, clusterInfo, auroraInfo, msInfo, dmsInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV[0].Count + base.AwsTopoConfigs.DMMaster.Count + base.AwsTopoConfigs.DMWorker.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t2 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t2)
	}

	if base.AwsAuroraConfigs.InstanceType != "" {
		t3 := task.NewBuilder().CreateAurora(&sexecutor, base.AwsWSConfigs, base.AwsAuroraConfigs, &auroraInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing aurora instance"))
		envInitTasks = append(envInitTasks, t3)
	}

	if base.AwsMSConfigs.InstanceType != "" {
		t4 := task.NewBuilder().CreateSqlServer(&sexecutor, "sqlserver", base.AwsMSConfigs, &msInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing sqlserver instance"))
		envInitTasks = append(envInitTasks, t4)
	}

	if base.AwsCloudFormationConfigs.TemplateBodyFilePath != "" || base.AwsCloudFormationConfigs.TemplateURL != "" {
		t5 := task.NewBuilder().CreateCloudFormation(&sexecutor, base.AwsCloudFormationConfigs, "", &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing cloud formation"))
		envInitTasks = append(envInitTasks, t5)
	}

	builder := task.NewBuilder().ParallelStep("+ Deploying all the sub components for tidb2ms solution service", false, envInitTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

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

	//	fmt.Printf("The data is <%d>\n\n\n", cntEC2Nodes)
	//	return nil
	var t5 *task.StepDisplay
	if cntEC2Nodes > 0 {
		t5 = task.NewBuilder().
			CreateDMSService(&sexecutor, "dmsservice", base.AwsDMSConfigs, &dmsInfo).
			CreateTransitGateway(&sexecutor).
			CreateTransitGatewayVpcAttachment(&sexecutor, "workstation").
			CreateTransitGatewayVpcAttachment(&sexecutor, "tidb").
			CreateTransitGatewayVpcAttachment(&sexecutor, "aurora").
			CreateTransitGatewayVpcAttachment(&sexecutor, "sqlserver").
			CreateTransitGatewayVpcAttachment(&sexecutor, "dmsservice").
			CreateRouteTgw(&sexecutor, "workstation", []string{"tidb", "aurora", "sqlserver", "dmsservice"}).
			CreateRouteTgw(&sexecutor, "tidb", []string{"aurora"}).
			CreateRouteTgw(&sexecutor, "dmsservice", []string{"aurora", "sqlserver"}).
			DeployTiDB(&sexecutor, "tidb", base.AwsWSConfigs, &workstationInfo).
			DeployTiDBInstance(&sexecutor, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo).
			//CreateTiDBNLB(&sexecutor, "tidb", &clusterInfo).
			//MakeDBObjects(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", &workstationInfo).
			DeployTiCDC(&sexecutor, "tidb", &workstationInfo). // - Set the TiCDC for data sync between TiDB and Aurora
			BuildAsStep(fmt.Sprintf("  - Prepare DMS servicer and additional network resources %s:%d", globalOptions.Host, 22))
	} else {
		t5 = task.NewBuilder().
			CreateDMSService(&sexecutor, "dmsservice", base.AwsDMSConfigs, &dmsInfo).
			CreateTransitGateway(&sexecutor).
			CreateTransitGatewayVpcAttachment(&sexecutor, "workstation").
			CreateTransitGatewayVpcAttachment(&sexecutor, "aurora").
			CreateTransitGatewayVpcAttachment(&sexecutor, "sqlserver").
			CreateTransitGatewayVpcAttachment(&sexecutor, "dmsservice").
			CreateRouteTgw(&sexecutor, "workstation", []string{"aurora", "sqlserver", "dmsservice"}).
			CreateRouteTgw(&sexecutor, "dmsservice", []string{"aurora", "sqlserver"}).
			DeployTiDB(&sexecutor, "tidb", base.AwsWSConfigs, &workstationInfo).
			CreateDMSTask(&sexecutor, "dmsservice", &dmsInfo).
			MakeDBObjects(&sexecutor, "tidb", &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))
	}

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	builder = task.NewBuilder().
		ParallelStep("+ Deploying tidb2ms solution service ... ...", false, t5)
	t = builder.Build()
	if err := t.Execute(ctxt.New(tailctx, gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	logger.OutputDebugLog("aws-nodes")
	return nil

	hint := color.New(color.Bold).Sprintf("%s start %s", tui.OsArgs0(), name)
	log.Infof("Cluster `%s` deployed successfully, you can start it with command: `%s`", name, hint)
	return nil
}
