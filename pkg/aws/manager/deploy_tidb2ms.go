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
	"github.com/luyomo/tisample/pkg/aws/clusterutil"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	//	"github.com/luyomo/tisample/pkg/crypto"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/logger"
	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	//	"github.com/luyomo/tisample/pkg/utils"
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

	instCnt := 0
	topo.IterInstance(func(inst spec.Instance) {
		switch inst.ComponentName() {
		// monitoring components are only useful when deployed with
		// core components, we do not support deploying any bare
		// monitoring system.
		case spec.ComponentGrafana,
			spec.ComponentPrometheus,
			spec.ComponentAlertmanager:
			return
		}
		instCnt++
	})
	if instCnt < 1 {
		return fmt.Errorf("no valid instance found in the input topology, please check your config")
	}

	spec.ExpandRelativeDir(topo)

	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	clusterList, err := m.specManager.GetAllClusters()
	if err != nil {
		return err
	}
	if err := spec.CheckClusterPortConflict(clusterList, name, topo); err != nil {
		return err
	}
	if err := spec.CheckClusterDirConflict(clusterList, name, topo); err != nil {
		return err
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

	var (
		envInitTasks []*task.StepDisplay // tasks which are used to initialize environment
		//downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		//deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	//	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	//	noAgentHosts := set.NewStringSet()
	globalOptions := base.GlobalOptions

	clusterType := "tisample-tidb2ms"
	var workstationInfo, clusterInfo, auroraInfo, msInfo, dmsInfo task.ClusterInfo
	t1 := task.NewBuilder().
		CreateWorkstationCluster(globalOptions.User, "127.0.0.1", name, clusterType, "workstation", base.AwsWSConfigs, &workstationInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing workstation"))
	fmt.Printf("%#v\n\n\n", t1)
	envInitTasks = append(envInitTasks, t1)

	t2 := task.NewBuilder().
		CreateTiDBCluster(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", base.AwsTopoConfigs, &clusterInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing tidb servers"))
	fmt.Printf("%#v\n\n\n", t2)
	envInitTasks = append(envInitTasks, t2)

	t3 := task.NewBuilder().
		CreateAurora(globalOptions.User, "127.0.0.1", name, clusterType, "aurora", base.AwsAuroraConfigs, &auroraInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing aurora instance"))
	fmt.Printf("%#v\n\n\n", t3)
	envInitTasks = append(envInitTasks, t3)

	t4 := task.NewBuilder().
		CreateSqlServer(globalOptions.User, "127.0.0.1", name, clusterType, "sqlserver", base.AwsMSConfigs, &msInfo).
		BuildAsStep(fmt.Sprintf("  - Preparing sqlserver instance"))
	fmt.Printf("%#v\n\n\n", t4)
	envInitTasks = append(envInitTasks, t4)

	//t4 := task.NewBuilder().
	//	CreateDMSService(globalOptions.User, "127.0.0.1", name, clusterType, "dmsservice", base.AwsDMSConfigs, &dmsInfo).
	//	BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))
	//envInitTasks = append(envInitTasks, t4)

	/*
					//CreateDMSTask(globalOptions.User, inst.GetHost(), name, clusterType).           // - Deploy the TiDB endpoint
					// - Deploy the Aurora endpoint
					// - Deploy DMS instance
					// - Deploy DMS task for data sync
					BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", inst.GetHost(), inst.GetSSHPort()))
				envInitTasks = append(envInitTasks, t)
			}
		})

		if iterErr != nil {
			return iterErr
		}
	*/

	builder := task.NewBuilder().
		//Step("+ Generate SSH keys",
		//Step("+ Generate SSH keys *****************************",
		//	task.NewBuilder().SSHKeyGen(m.specManager.Path(name, "ssh", "id_rsa")).Build()).
		ParallelStep("+ Initialize target host environments", false, envInitTasks...)
		//ParallelStep("+ Download TiDB components", false, downloadCompTasks...).
	//ParallelStep("+ Copy files", false, deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

	t := builder.Build()

	//if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
	//	if errorx.Cast(err) != nil {
	//		// FIXME: Map possible task errors and give suggestions.
	//		return err
	//	}
	//	return err
	//}

	t5 := task.NewBuilder().
		//CreateDMSService(globalOptions.User, "127.0.0.1", name, clusterType, "dmsservice", base.AwsDMSConfigs, &dmsInfo).
		//CreateTransitGateway(globalOptions.User, "127.0.0.1", name, clusterType).
		//CreateTransitGatewayVpcAttachment(globalOptions.User, "127.0.0.1", name, clusterType, "workstation").
		//CreateTransitGatewayVpcAttachment(globalOptions.User, "127.0.0.1", name, clusterType, "tidb").
		//CreateTransitGatewayVpcAttachment(globalOptions.User, "127.0.0.1", name, clusterType, "aurora").
		//CreateTransitGatewayVpcAttachment(globalOptions.User, "127.0.0.1", name, clusterType, "sqlserver").
		//CreateTransitGatewayVpcAttachment(globalOptions.User, "127.0.0.1", name, clusterType, "dmsservice").
		//CreateRouteTgw(globalOptions.User, "127.0.0.1", name, clusterType, "workstation", []string{"tidb", "aurora", "sqlserver", "dmsservice"}).
		//CreateRouteTgw(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", []string{"aurora"}).
		//CreateRouteTgw(globalOptions.User, "127.0.0.1", name, clusterType, "dmsservice", []string{"aurora", "sqlserver"}).
		DeployTiDB(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", base.AwsTopoConfigs, &workstationInfo).
		DeployTiDBInstance(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", &workstationInfo).
		MakeDBObjects(globalOptions.User, "127.0.0.1", name, clusterType, "tidb", &workstationInfo).
		//DeployTiCDC(globalOptions.User, inst.GetHost(), name, clusterType).        // - Set the TiCDC for data sync between TiDB and Aurora
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder = task.NewBuilder().
		ParallelStep("+ Initialize target host environments", false, t5)
	t = builder.Build()
	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	logger.OutputDebugLog("aws-nodes")
	return nil

	fmt.Printf("dmsinfo is <%#v> \n", dmsInfo)

	hint := color.New(color.Bold).Sprintf("%s start %s", tui.OsArgs0(), name)
	log.Infof("Cluster `%s` deployed successfully, you can start it with command: `%s`", name, hint)
	return nil
}
