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

	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
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
}

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
			{"Component", "# of nodes", "Instance Type", "Image Name", "CIDR", "User"},
		}
		clusterTable = append(clusterTable, []string{"Workstation", "1", spec.AwsWSConfigs.InstanceType, spec.AwsWSConfigs.ImageId, spec.AwsWSConfigs.CIDR, "admin"})

		if spec.AwsTopoConfigs.TiDB.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiDB", strconv.Itoa(spec.AwsTopoConfigs.TiDB.Count), spec.AwsTopoConfigs.TiDB.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.PD.Count > 0 {
			clusterTable = append(clusterTable, []string{"PD", strconv.Itoa(spec.AwsTopoConfigs.PD.Count), spec.AwsTopoConfigs.PD.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.TiKV.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiKV", strconv.Itoa(spec.AwsTopoConfigs.TiKV.Count), spec.AwsTopoConfigs.TiKV.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.TiCDC.Count > 0 {
			clusterTable = append(clusterTable, []string{"TiCDC", strconv.Itoa(spec.AwsTopoConfigs.TiCDC.Count), spec.AwsTopoConfigs.TiCDC.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
		}

		if spec.AwsTopoConfigs.DM.Count > 0 {
			clusterTable = append(clusterTable, []string{"DM", strconv.Itoa(spec.AwsTopoConfigs.DM.Count), spec.AwsTopoConfigs.DM.InstanceType, spec.AwsTopoConfigs.General.ImageId, spec.AwsTopoConfigs.General.CIDR, "master"})
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
