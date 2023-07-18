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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"

	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	perrs "github.com/pingcap/errors"
)

var (
	errNSDeploy            = errorx.NewNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", utils.ErrTraitPreCheck)

	errNSRename              = errorx.NewNamespace("rename")
	errorRenameNameNotExist  = errNSRename.NewType("name_not_exist", utils.ErrTraitPreCheck)
	errorRenameNameDuplicate = errNSRename.NewType("name_dup", utils.ErrTraitPreCheck)
)

// Manager to deploy a cluster.
type Manager struct {
	sysName     string
	specManager *spec.SpecManager
	bindVersion spec.BindVersion
	wsExe       ctxt.Executor
	localExe    ctxt.Executor
	workstation ws.Workstation
}

type INC_WS_FLAG bool

const (
	INC_WS INC_WS_FLAG = true
	EXC_WS INC_WS_FLAG = false
)

// NewManager create a Manager.
func NewManager(sysName string, specManager *spec.SpecManager, bindVersion spec.BindVersion) *Manager {
	return &Manager{
		sysName:     sysName,
		specManager: specManager,
		bindVersion: bindVersion,
	}
}

func (m *Manager) meta(name string) (metadata spec.Metadata, err error) {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})

	stdout, _, err := local.Execute(ctxt.New(context.Background(), 1), fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\"", name), false)
	if err != nil {
		return nil, err
	}
	var vpcs task.Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil, err
	}
	if len(vpcs.Vpcs) == 0 {
		return nil, perrs.Errorf("Cluster `%s` not exists", name)
	}

	return nil, nil
}

func (m *Manager) confirmTopology(name, version string, topo spec.Topology, patchedRoles set.StringSet) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)

	if spec, ok := topo.(*spec.Specification); ok {
		fmt.Printf("AWS Region:      %s\n", cyan.Sprint(spec.AwsTopoConfigs.General.Region))
		fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
		fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
		fmt.Printf("Cluster version: %s\n", cyan.Sprint(spec.AwsTopoConfigs.General.TiDBVersion))
		fmt.Printf("User Name:       %s\n", cyan.Sprint(spec.AwsTopoConfigs.General.Name))
		fmt.Printf("Key Name:        %s\n", cyan.Sprint(spec.AwsTopoConfigs.General.KeyName))
		fmt.Printf("\n")

		clusterTable := [][]string{
			// Header
			{"Component", "# of nodes", "Instance Type", "Image Name", "CIDR", "User", "Placement rule labels"},
		}
		if spec.AwsWSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"Workstation", "1", spec.AwsWSConfigs.InstanceType, spec.AwsWSConfigs.ImageId, spec.AwsWSConfigs.CIDR, "admin"})
		}

		if spec.AwsTopoConfigs.TiDB.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiDB", strconv.Itoa(spec.AwsTopoConfigs.TiDB.Count), spec.AwsTopoConfigs.TiDB.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.PD.Count > 0 {
			clusterTable = append(clusterTable, []string{"PD", strconv.Itoa(spec.AwsTopoConfigs.PD.Count), spec.AwsTopoConfigs.PD.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		for _, tikvGroup := range spec.AwsTopoConfigs.TiKV {
			clusterTable = append(clusterTable, []string{"TiKV", strconv.Itoa(tikvGroup.Count), tikvGroup.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.TiFlash.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiFlash", strconv.Itoa(spec.AwsTopoConfigs.TiFlash.Count), spec.AwsTopoConfigs.TiFlash.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.TiCDC.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiCDC", strconv.Itoa(spec.AwsTopoConfigs.TiCDC.Count), spec.AwsTopoConfigs.TiCDC.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.DMMaster.Count > 0 {
			clusterTable = append(clusterTable, []string{"DM Master", strconv.Itoa(spec.AwsTopoConfigs.DMMaster.Count), spec.AwsTopoConfigs.DMMaster.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}
		if spec.AwsTopoConfigs.DMWorker.Count > 0 {
			clusterTable = append(clusterTable, []string{"DM Worker", strconv.Itoa(spec.AwsTopoConfigs.DMWorker.Count), spec.AwsTopoConfigs.DMWorker.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.Pump.Count > 0 {
			clusterTable = append(clusterTable, []string{"Pump", strconv.Itoa(spec.AwsTopoConfigs.Pump.Count), spec.AwsTopoConfigs.Pump.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.Drainer.Count > 0 {
			clusterTable = append(clusterTable, []string{"Drainer", strconv.Itoa(spec.AwsTopoConfigs.Drainer.Count), spec.AwsTopoConfigs.Drainer.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsAuroraConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"Aurora", "1", spec.AwsAuroraConfigs.InstanceType, "-", spec.AwsAuroraConfigs.CIDR, "master"})
		}

		if spec.AwsMSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"MSSQLServer", "1", spec.AwsMSConfigs.InstanceType, "-", spec.AwsMSConfigs.CIDR, "-"})
		}

		if spec.AwsDMSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"DMS", "1", spec.AwsDMSConfigs.InstanceType, "-", spec.AwsDMSConfigs.CIDR, "-"})
		}

		if spec.AwsKafkaTopoConfigs.Zookeeper.Count > 0 {
			clusterTable = append(clusterTable, []string{"Zookeeper", strconv.Itoa(spec.AwsKafkaTopoConfigs.Zookeeper.Count), spec.AwsKafkaTopoConfigs.Zookeeper.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.Broker.Count > 0 {
			clusterTable = append(clusterTable, []string{"Broker", strconv.Itoa(spec.AwsKafkaTopoConfigs.Broker.Count), spec.AwsKafkaTopoConfigs.Broker.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.SchemaRegistry.Count > 0 {
			clusterTable = append(clusterTable, []string{"Schema Registry", strconv.Itoa(spec.AwsKafkaTopoConfigs.SchemaRegistry.Count), spec.AwsKafkaTopoConfigs.SchemaRegistry.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.RestService.Count > 0 {
			clusterTable = append(clusterTable, []string{"Rest Service", strconv.Itoa(spec.AwsKafkaTopoConfigs.RestService.Count), spec.AwsKafkaTopoConfigs.RestService.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.Connector.Count > 0 {
			clusterTable = append(clusterTable, []string{"Connector", strconv.Itoa(spec.AwsKafkaTopoConfigs.Connector.Count), spec.AwsKafkaTopoConfigs.Connector.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		tui.PrintTable(clusterTable, true)
	}

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) confirmKafkaTopology(name string, topo spec.Topology) error {
	log.Infof("Please confirm your kafka topology:")

	cyan := color.New(color.FgCyan, color.Bold)

	if spec, ok := topo.(*spec.Specification); ok {
		fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
		fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
		fmt.Printf("Cluster version: %s\n", cyan.Sprint(spec.AwsKafkaTopoConfigs.General.TiDBVersion))
		fmt.Printf("\n")

		clusterTable := [][]string{
			// Header
			{"Component", "# of nodes", "Instance Type", "Image Name", "CIDR", "User", "Placement rule labels"},
		}
		if spec.AwsWSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"Workstation", "1", spec.AwsWSConfigs.InstanceType, spec.AwsWSConfigs.ImageId, spec.AwsWSConfigs.CIDR, "admin"})
		}

		if spec.AwsKafkaTopoConfigs.Zookeeper.Count > 0 {
			clusterTable = append(clusterTable, []string{"Zookeeper", strconv.Itoa(spec.AwsKafkaTopoConfigs.Zookeeper.Count), spec.AwsKafkaTopoConfigs.Zookeeper.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.Broker.Count > 0 {
			clusterTable = append(clusterTable, []string{"Broker", strconv.Itoa(spec.AwsKafkaTopoConfigs.Broker.Count), spec.AwsKafkaTopoConfigs.Broker.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.SchemaRegistry.Count > 0 {
			clusterTable = append(clusterTable, []string{"Schema Registry", strconv.Itoa(spec.AwsKafkaTopoConfigs.SchemaRegistry.Count), spec.AwsKafkaTopoConfigs.SchemaRegistry.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.RestService.Count > 0 {
			clusterTable = append(clusterTable, []string{"Rest Service", strconv.Itoa(spec.AwsKafkaTopoConfigs.RestService.Count), spec.AwsKafkaTopoConfigs.RestService.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsKafkaTopoConfigs.Connector.Count > 0 {
			clusterTable = append(clusterTable, []string{"Connector", strconv.Itoa(spec.AwsKafkaTopoConfigs.Connector.Count), spec.AwsKafkaTopoConfigs.Connector.InstanceType, spec.AwsKafkaTopoConfigs.General.ImageId, spec.AwsKafkaTopoConfigs.General.CIDR, "master"})
		}

		tui.PrintTable(clusterTable, true)
	}

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) confirmMongoTopology(name string, topo spec.Topology) error {
	log.Infof("Please confirm your mongo topology:")

	cyan := color.New(color.FgCyan, color.Bold)

	if spec, ok := topo.(*spec.Specification); ok {
		fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
		fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
		fmt.Printf("Cluster version: %s\n", cyan.Sprint(spec.AwsMongoTopoConfigs.General.TiDBVersion))
		fmt.Printf("\n")

		clusterTable := [][]string{
			// Header
			{"Component", "# of nodes", "Instance Type", "Image Name", "CIDR", "User"},
		}
		if spec.AwsWSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"Workstation", "1", spec.AwsWSConfigs.InstanceType, spec.AwsWSConfigs.ImageId, spec.AwsWSConfigs.CIDR, "admin"})
		}

		if spec.AwsMongoTopoConfigs.ConfigServer.Count > 0 {
			clusterTable = append(clusterTable, []string{"Config Server", strconv.Itoa(spec.AwsMongoTopoConfigs.ConfigServer.Count), spec.AwsMongoTopoConfigs.ConfigServer.InstanceType, spec.AwsMongoTopoConfigs.General.ImageId, spec.AwsMongoTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsMongoTopoConfigs.Mongos.Count > 0 {
			clusterTable = append(clusterTable, []string{"Mongos", strconv.Itoa(spec.AwsMongoTopoConfigs.Mongos.Count), spec.AwsMongoTopoConfigs.Mongos.InstanceType, spec.AwsMongoTopoConfigs.General.ImageId, spec.AwsMongoTopoConfigs.General.CIDR, "master"})
		}

		for _, replicaSet := range spec.AwsMongoTopoConfigs.ReplicaSet {
			clusterTable = append(clusterTable, []string{"Replica Set", strconv.Itoa(replicaSet.Count), replicaSet.InstanceType, spec.AwsMongoTopoConfigs.General.ImageId, spec.AwsMongoTopoConfigs.General.CIDR, "master"})
		}

		tui.PrintTable(clusterTable, true)
	}

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) confirmTiDBTopology(name string, topo spec.Topology) error {
	log.Infof("Please confirm your TiDB topology:")

	cyan := color.New(color.FgCyan, color.Bold)

	if spec, ok := topo.(*spec.Specification); ok {
		generalCfg := spec.AwsTopoConfigs.General
		clusterCfg := spec.AwsTopoConfigs

		fmt.Printf("AWS Region:      %s\n", cyan.Sprint(generalCfg.Region))
		fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
		fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
		fmt.Printf("Cluster version: %s\n", cyan.Sprint(generalCfg.TiDBVersion))
		fmt.Printf("User Name:       %s\n", cyan.Sprint(generalCfg.Name))
		fmt.Printf("Key Name:        %s\n", cyan.Sprint(generalCfg.KeyName))
		fmt.Printf("\n")

		clusterTable := [][]string{
			// Header
			{"Component", "# of nodes", "Instance Type", "Image Name", "CIDR", "User", "Placement rule labels"},
		}
		if spec.AwsWSConfigs.InstanceType != "" {
			clusterTable = append(clusterTable, []string{"Workstation", "1", spec.AwsWSConfigs.InstanceType, spec.AwsWSConfigs.ImageId, spec.AwsWSConfigs.CIDR, "admin"})
		}

		if clusterCfg.TiDB.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiDB", strconv.Itoa(clusterCfg.TiDB.Count), clusterCfg.TiDB.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.PD.Count > 0 {
			clusterTable = append(clusterTable, []string{"PD", strconv.Itoa(clusterCfg.PD.Count), clusterCfg.PD.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		for _, tikvGroup := range spec.AwsTopoConfigs.TiKV {
			clusterTable = append(clusterTable, []string{"TiKV", strconv.Itoa(tikvGroup.Count), tikvGroup.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if clusterCfg.TiFlash.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiFlash", strconv.Itoa(clusterCfg.TiFlash.Count), clusterCfg.TiFlash.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.TiCDC.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiCDC", strconv.Itoa(clusterCfg.TiCDC.Count), clusterCfg.TiCDC.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.DMMaster.Count > 0 {
			clusterTable = append(clusterTable, []string{"DM Master", strconv.Itoa(clusterCfg.DMMaster.Count), clusterCfg.DMMaster.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}
		if clusterCfg.DMWorker.Count > 0 {
			clusterTable = append(clusterTable, []string{"DM Worker", strconv.Itoa(clusterCfg.DMWorker.Count), clusterCfg.DMWorker.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.Pump.Count > 0 {
			clusterTable = append(clusterTable, []string{"Pump", strconv.Itoa(clusterCfg.Pump.Count), clusterCfg.Pump.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.Drainer.Count > 0 {
			clusterTable = append(clusterTable, []string{"Drainer", strconv.Itoa(clusterCfg.Drainer.Count), clusterCfg.Drainer.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.Monitor.Count > 0 {
			clusterTable = append(clusterTable, []string{"Monitor", strconv.Itoa(clusterCfg.Monitor.Count), clusterCfg.Monitor.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.Grafana.Count > 0 {
			clusterTable = append(clusterTable, []string{"Grafana", strconv.Itoa(clusterCfg.Grafana.Count), clusterCfg.Grafana.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		if clusterCfg.AlertManager.Count > 0 {
			clusterTable = append(clusterTable, []string{"Alert Manager", strconv.Itoa(clusterCfg.AlertManager.Count), clusterCfg.AlertManager.InstanceType, generalCfg.ImageId, generalCfg.CIDR, "master"})
		}

		tui.PrintTable(clusterTable, true)
	}

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) sshTaskBuilder(name string, topo spec.Topology, user string, gOpt operator.Options) (*task.Builder, error) {
	var p *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone && len(gOpt.SSHProxyHost) != 0 {
		var err error
		if p, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
			return nil, err
		}
	}

	return task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(name, "ssh", "id_rsa"),
			m.specManager.Path(name, "ssh", "id_rsa.pub"),
		).
		ClusterSSH(
			topo,
			user,
			gOpt.SSHTimeout,
			gOpt.OptTimeout,
			gOpt.SSHProxyHost,
			gOpt.SSHProxyPort,
			gOpt.SSHProxyUser,
			p.Password,
			p.IdentityFile,
			p.IdentityFilePassphrase,
			gOpt.SSHProxyTimeout,
			gOpt.SSHType,
			topo.BaseTopo().GlobalOptions.SSHType,
		), nil
}

func (m *Manager) fillHostArch(s, p *tui.SSHConnectionProps, topo spec.Topology, gOpt *operator.Options, user string) error {
	globalSSHType := topo.BaseTopo().GlobalOptions.SSHType
	hostArch := map[string]string{}
	var detectTasks []*task.StepDisplay
	topo.IterInstance(func(inst spec.Instance) {
		if _, ok := hostArch[inst.GetHost()]; ok {
			return
		}
		hostArch[inst.GetHost()] = ""
		if inst.Arch() != "" {
			return
		}

		tf := task.NewBuilder().
			RootSSH(
				inst.GetHost(),
				inst.GetSSHPort(),
				user,
				s.Password,
				s.IdentityFile,
				s.IdentityFilePassphrase,
				gOpt.SSHTimeout,
				gOpt.OptTimeout,
				gOpt.SSHProxyHost,
				gOpt.SSHProxyPort,
				gOpt.SSHProxyUser,
				p.Password,
				p.IdentityFile,
				p.IdentityFilePassphrase,
				gOpt.SSHProxyTimeout,
				gOpt.SSHType,
				globalSSHType,
			).
			Shell(inst.GetHost(), "uname -m", "", false).
			BuildAsStep(fmt.Sprintf("  - Detecting node %s", inst.GetHost()))
		detectTasks = append(detectTasks, tf)
	})
	if len(detectTasks) == 0 {
		return nil
	}

	ctx := ctxt.New(context.Background(), gOpt.Concurrency)
	t := task.NewBuilder().
		ParallelStep("+ Detect CPU Arch", false, detectTasks...).
		Build()

	if err := t.Execute(ctx); err != nil {
		return perrs.Annotate(err, "failed to fetch cpu arch")
	}

	for host := range hostArch {
		stdout, _, ok := ctxt.GetInner(ctx).GetOutputs(host)
		if !ok {
			return fmt.Errorf("no check results found for %s", host)
		}
		hostArch[host] = strings.Trim(string(stdout), "\n")
	}
	return topo.FillHostArch(hostArch)
}

type ConfigData struct {
	User                string `yaml:"user"`
	IdentityFile        string `yaml:"identity-file"`
	TiDBCloudPrivateKey string `yaml:"tidbcloud-private-key"`
	TiDBCloudPublicKey  string `yaml:"tidbcloud-public-key"`
}

func (m *Manager) makeExeContext(ctx context.Context, topo *spec.Topology, gOpt *operator.Options, includeWS INC_WS_FLAG, awsCliFlag ws.INC_AWS_ENV_FLAG) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	var err error
	if m.localExe == nil {
		m.localExe, err = executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
		if err != nil {
			return err
		}
	}

	if includeWS != INC_WS {
		return nil
	}

	pWS, err := ws.NewAWSWorkstation(&m.localExe, clusterName, clusterType, (*gOpt).SSHUser, (*gOpt).IdentityFile, awsCliFlag)
	if err != nil {
		return err
	}

	m.workstation = *pWS

	pWsExe, err := m.workstation.GetExecutor()
	if err != nil {
		return err
	}
	m.wsExe = *pWsExe

	if err := m.workstation.InstallProfiles(); err != nil {
		return err
	}

	return nil
}
