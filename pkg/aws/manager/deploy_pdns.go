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
	//	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/luyomo/tisample/pkg/aws/clusterutil"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	//	"github.com/luyomo/tisample/pkg/crypto"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/logger"
	//	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	//	"go.uber.org/zap"
	"os"
	//"strings"
)

// DeployOptions contains the options for scale out.
type PDNSDeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (m *Manager) PDNSDeploy(
	name string,
	topoFile string,
	opt PDNSDeployOptions,
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

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()})
	if err != nil {
		return err
	}
	clusterType := "ohmytiup-pdns"

	var workstationInfo, clusterInfo task.ClusterInfo

	if base.AwsWSConfigs.InstanceType != "" {
		t1 := task.NewBuilder().CreateWorkstationCluster(&sexecutor, "workstation", base.AwsWSConfigs, &workstationInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing workstation"))

		envInitTasks = append(envInitTasks, t1)
	}

	cntEC2Nodes := base.AwsTopoConfigs.PD.Count + base.AwsTopoConfigs.TiDB.Count + base.AwsTopoConfigs.TiKV.Count + base.AwsTopoConfigs.DM.Count + base.AwsTopoConfigs.TiCDC.Count
	if cntEC2Nodes > 0 {
		t2 := task.NewBuilder().CreateTiDBCluster(&sexecutor, "tidb", base.AwsTopoConfigs, &clusterInfo).
			BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
		envInitTasks = append(envInitTasks, t2)
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

	var t5 *task.StepDisplay

	t5 = task.NewBuilder().
		CreateTransitGateway(&sexecutor).
		CreateTransitGatewayVpcAttachment(&sexecutor, "workstation").
		CreateTransitGatewayVpcAttachment(&sexecutor, "tidb").
		CreateRouteTgw(&sexecutor, "workstation", []string{"tidb"}).
		DeployTiDB(&sexecutor, "tidb", base.AwsWSConfigs, &workstationInfo).
		DeployTiDBInstance(&sexecutor, base.AwsWSConfigs, "tidb", base.AwsTopoConfigs.General.TiDBVersion, &workstationInfo).
		CreateTiDBNLB(&sexecutor, "tidb", &clusterInfo).
		DeployPDNS(&sexecutor, "tidb", base.AwsWSConfigs).
		DeployWS(&sexecutor, "tidb", base.AwsWSConfigs).
		BuildAsStep(fmt.Sprintf("  - Deploying PDNS service %s:%d", globalOptions.Host, 22))

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
}
