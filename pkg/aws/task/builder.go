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

package task

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"

	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/crypto"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/proxy"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// Builder is used to build TiUP task
type Builder struct {
	tasks []Task
}

// NewBuilder returns a *Builder instance
func NewBuilder() *Builder {
	return &Builder{}
}

// RootSSH appends a RootSSH task to the current task collection
func (b *Builder) RootSSH(
	host string, port int, user, password, keyFile, passphrase string, sshTimeout, exeTimeout uint64,
	proxyHost string, proxyPort int, proxyUser, proxyPassword, proxyKeyFile, proxyPassphrase string, proxySSHTimeout uint64,
	sshType, defaultSSHType executor.SSHType,
) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	b.tasks = append(b.tasks, &RootSSH{
		host:            host,
		port:            port,
		user:            user,
		password:        password,
		keyFile:         keyFile,
		passphrase:      passphrase,
		timeout:         sshTimeout,
		exeTimeout:      exeTimeout,
		proxyHost:       proxyHost,
		proxyPort:       proxyPort,
		proxyUser:       proxyUser,
		proxyPassword:   proxyPassword,
		proxyKeyFile:    proxyKeyFile,
		proxyPassphrase: proxyPassphrase,
		proxyTimeout:    proxySSHTimeout,
		sshType:         sshType,
	})
	return b
}

// UserSSH append a UserSSH task to the current task collection
func (b *Builder) UserSSH(
	host string, port int, deployUser string, sshTimeout, exeTimeout uint64,
	proxyHost string, proxyPort int, proxyUser, proxyPassword, proxyKeyFile, proxyPassphrase string, proxySSHTimeout uint64,
	sshType, defaultSSHType executor.SSHType,
) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	b.tasks = append(b.tasks, &UserSSH{
		host:            host,
		port:            port,
		deployUser:      deployUser,
		timeout:         sshTimeout,
		exeTimeout:      exeTimeout,
		proxyHost:       proxyHost,
		proxyPort:       proxyPort,
		proxyUser:       proxyUser,
		proxyPassword:   proxyPassword,
		proxyKeyFile:    proxyKeyFile,
		proxyPassphrase: proxyPassphrase,
		proxyTimeout:    proxySSHTimeout,
		sshType:         sshType,
	})
	return b
}

// Func append a func task.
func (b *Builder) Func(name string, fn func(ctx context.Context) error) *Builder {
	b.tasks = append(b.tasks, &Func{
		name: name,
		fn:   fn,
	})
	return b
}

// ClusterSSH init all UserSSH need for the cluster.
func (b *Builder) ClusterSSH(
	topo spec.Topology,
	deployUser string, sshTimeout, exeTimeout uint64,
	proxyHost string, proxyPort int, proxyUser, proxyPassword, proxyKeyFile, proxyPassphrase string, proxySSHTimeout uint64,
	sshType, defaultSSHType executor.SSHType,
) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	var tasks []Task
	topo.IterInstance(func(inst spec.Instance) {
		tasks = append(tasks, &UserSSH{
			host:            inst.GetHost(),
			port:            inst.GetSSHPort(),
			deployUser:      deployUser,
			timeout:         sshTimeout,
			exeTimeout:      exeTimeout,
			proxyHost:       proxyHost,
			proxyPort:       proxyPort,
			proxyUser:       proxyUser,
			proxyPassword:   proxyPassword,
			proxyKeyFile:    proxyKeyFile,
			proxyPassphrase: proxyPassphrase,
			proxyTimeout:    proxySSHTimeout,
			sshType:         sshType,
		})
	})

	b.tasks = append(b.tasks, &Parallel{inner: tasks})

	return b
}

// UpdateMeta maintain the meta information
func (b *Builder) UpdateMeta(cluster string, metadata *spec.ClusterMeta, deletedNodeIds []string) *Builder {
	b.tasks = append(b.tasks, &UpdateMeta{
		cluster:        cluster,
		metadata:       metadata,
		deletedNodeIDs: deletedNodeIds,
	})
	return b
}

// UpdateTopology maintain the topology information
func (b *Builder) UpdateTopology(cluster, profile string, metadata *spec.ClusterMeta, deletedNodeIds []string) *Builder {
	b.tasks = append(b.tasks, &UpdateTopology{
		metadata:       metadata,
		cluster:        cluster,
		profileDir:     profile,
		deletedNodeIDs: deletedNodeIds,
		tcpProxy:       proxy.GetTCPProxy(),
	})
	return b
}

// CopyFile appends a CopyFile task to the current task collection
func (b *Builder) CopyFile(src, dst, server string, download bool, limit int) *Builder {
	b.tasks = append(b.tasks, &CopyFile{
		src:      src,
		dst:      dst,
		remote:   server,
		download: download,
		limit:    limit,
	})
	return b
}

// Download appends a Downloader task to the current task collection
func (b *Builder) Download(component, os, arch string, version string) *Builder {
	b.tasks = append(b.tasks, NewDownloader(component, os, arch, version))
	return b
}

// CopyComponent appends a CopyComponent task to the current task collection
func (b *Builder) CopyComponent(component, os, arch string,
	version string,
	srcPath, dstHost, dstDir string,
) *Builder {
	b.tasks = append(b.tasks, &CopyComponent{
		component: component,
		os:        os,
		arch:      arch,
		version:   version,
		srcPath:   srcPath,
		host:      dstHost,
		dstDir:    dstDir,
	})
	return b
}

// InstallPackage appends a InstallPackage task to the current task collection
func (b *Builder) InstallPackage(srcPath, dstHost, dstDir string) *Builder {
	b.tasks = append(b.tasks, &InstallPackage{
		srcPath: srcPath,
		host:    dstHost,
		dstDir:  dstDir,
	})
	return b
}

// BackupComponent appends a BackupComponent task to the current task collection
func (b *Builder) BackupComponent(component, fromVer string, host, deployDir string) *Builder {
	b.tasks = append(b.tasks, &BackupComponent{
		component: component,
		fromVer:   fromVer,
		host:      host,
		deployDir: deployDir,
	})
	return b
}

// InitConfig appends a CopyComponent task to the current task collection
func (b *Builder) InitConfig(clusterName, clusterVersion string, specManager *spec.SpecManager, inst spec.Instance, deployUser string, ignoreCheck bool, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &InitConfig{
		specManager:    specManager,
		clusterName:    clusterName,
		clusterVersion: clusterVersion,
		instance:       inst,
		deployUser:     deployUser,
		ignoreCheck:    ignoreCheck,
		paths:          paths,
	})
	return b
}

// ScaleConfig generate temporary config on scaling
func (b *Builder) ScaleConfig(clusterName, clusterVersion string, specManager *spec.SpecManager, topo spec.Topology, inst spec.Instance, deployUser string, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &ScaleConfig{
		specManager:    specManager,
		clusterName:    clusterName,
		clusterVersion: clusterVersion,
		base:           topo,
		instance:       inst,
		deployUser:     deployUser,
		paths:          paths,
	})
	return b
}

// MonitoredConfig appends a CopyComponent task to the current task collection
func (b *Builder) MonitoredConfig(name, comp, host string, globResCtl meta.ResourceControl, options *spec.MonitoredOptions, deployUser string, tlsEnabled bool, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &MonitoredConfig{
		name:       name,
		component:  comp,
		host:       host,
		globResCtl: globResCtl,
		options:    options,
		deployUser: deployUser,
		tlsEnabled: tlsEnabled,
		paths:      paths,
	})
	return b
}

// SSHKeyGen appends a SSHKeyGen task to the current task collection
func (b *Builder) SSHKeyGen(keypath string) *Builder {
	b.tasks = append(b.tasks, &SSHKeyGen{
		keypath: keypath,
	})
	return b
}

// SSHKeySet appends a SSHKeySet task to the current task collection
func (b *Builder) SSHKeySet(privKeyPath, pubKeyPath string) *Builder {
	b.tasks = append(b.tasks, &SSHKeySet{
		privateKeyPath: privKeyPath,
		publicKeyPath:  pubKeyPath,
	})
	return b
}

// EnvInit appends a EnvInit task to the current task collection
func (b *Builder) EnvInit(host, deployUser string, userGroup string, skipCreateUser bool) *Builder {
	b.tasks = append(b.tasks, &EnvInit{
		host:           host,
		deployUser:     deployUser,
		userGroup:      userGroup,
		skipCreateUser: skipCreateUser,
	})
	return b
}

// ClusterOperate appends a cluster operation task.
// All the UserSSH needed must be init first.
func (b *Builder) ClusterOperate(
	spec *spec.Specification,
	op operator.Operation,
	options operator.Options,
	tlsCfg *tls.Config,
) *Builder {
	b.tasks = append(b.tasks, &ClusterOperate{
		spec:    spec,
		op:      op,
		options: options,
		tlsCfg:  tlsCfg,
	})

	return b
}

// Mkdir appends a Mkdir task to the current task collection
func (b *Builder) Mkdir(user, host string, dirs ...string) *Builder {
	b.tasks = append(b.tasks, &Mkdir{
		user: user,
		host: host,
		dirs: dirs,
	})
	return b
}

// Rmdir appends a Rmdir task to the current task collection
func (b *Builder) Rmdir(host string, dirs ...string) *Builder {
	b.tasks = append(b.tasks, &Rmdir{
		host: host,
		dirs: dirs,
	})
	return b
}

// Shell command on cluster host
func (b *Builder) Shell(host, command, cmdID string, sudo bool) *Builder {
	b.tasks = append(b.tasks, &Shell{
		host:    host,
		command: command,
		sudo:    sudo,
		cmdID:   cmdID,
	})
	return b
}

// SystemCtl run systemctl on host
func (b *Builder) SystemCtl(host, unit, action string, daemonReload bool) *Builder {
	b.tasks = append(b.tasks, &SystemCtl{
		host:         host,
		unit:         unit,
		action:       action,
		daemonReload: daemonReload,
	})
	return b
}

// Sysctl set a kernel parameter
func (b *Builder) Sysctl(host, key, val string) *Builder {
	b.tasks = append(b.tasks, &Sysctl{
		host: host,
		key:  key,
		val:  val,
	})
	return b
}

// Limit set a system limit
func (b *Builder) Limit(host, domain, limit, item, value string) *Builder {
	b.tasks = append(b.tasks, &Limit{
		host:   host,
		domain: domain,
		limit:  limit,
		item:   item,
		value:  value,
	})
	return b
}

// CheckSys checks system information of deploy server
func (b *Builder) CheckSys(host, dir, checkType string, topo *spec.Specification, opt *operator.CheckOptions) *Builder {
	b.tasks = append(b.tasks, &CheckSys{
		host:     host,
		topo:     topo,
		opt:      opt,
		checkDir: dir,
		check:    checkType,
	})
	return b
}

// DeploySpark deployes spark as dependency of TiSpark
func (b *Builder) DeploySpark(inst spec.Instance, sparkVersion, srcPath, deployDir string) *Builder {
	sparkSubPath := spec.ComponentSubDir(spec.ComponentSpark, sparkVersion)
	return b.CopyComponent(
		spec.ComponentSpark,
		inst.OS(),
		inst.Arch(),
		sparkVersion,
		srcPath,
		inst.GetHost(),
		deployDir,
	).Shell( // spark is under a subdir, move it to deploy dir
		inst.GetHost(),
		fmt.Sprintf(
			"cp -rf %[1]s %[2]s/ && cp -rf %[3]s/* %[2]s/ && rm -rf %[1]s %[3]s",
			filepath.Join(deployDir, "bin", sparkSubPath),
			deployDir,
			filepath.Join(deployDir, sparkSubPath),
		),
		"",
		false, // (not) sudo
	).CopyComponent(
		inst.ComponentName(),
		inst.OS(),
		inst.Arch(),
		"", // use the latest stable version
		srcPath,
		inst.GetHost(),
		deployDir,
	).Shell( // move tispark jar to correct path
		inst.GetHost(),
		fmt.Sprintf(
			"cp -f %[1]s/*.jar %[2]s/jars/ && rm -f %[1]s/*.jar",
			filepath.Join(deployDir, "bin"),
			deployDir,
		),
		"",
		false, // (not) sudo
	)
}

// TLSCert generates certificate for instance and transfers it to the server
func (b *Builder) TLSCert(host, comp, role string, port int, ca *crypto.CertificateAuthority, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &TLSCert{
		host:  host,
		comp:  comp,
		role:  role,
		port:  port,
		ca:    ca,
		paths: paths,
	})
	return b
}

// Parallel appends a parallel task to the current task collection
func (b *Builder) Parallel(ignoreError bool, tasks ...Task) *Builder {
	if len(tasks) > 0 {
		b.tasks = append(b.tasks, &Parallel{ignoreError: ignoreError, inner: tasks})
	}
	return b
}

// Serial appends the tasks to the tail of queue
func (b *Builder) Serial(tasks ...Task) *Builder {
	if len(tasks) > 0 {
		b.tasks = append(b.tasks, tasks...)
	}
	return b
}

// Build returns a task that contains all tasks appended by previous operation
func (b *Builder) Build() Task {
	// Serial handles event internally. So the following 3 lines are commented out.
	// if len(b.tasks) == 1 {
	//  return b.tasks[0]
	// }
	return &Serial{inner: b.tasks}
}

// Step appends a new StepDisplay task, which will print single line progress for inner tasks.
func (b *Builder) Step(prefix string, inner Task) *Builder {
	b.Serial(newStepDisplay(prefix, inner))
	return b
}

// ParallelStep appends a new ParallelStepDisplay task, which will print multi line progress in parallel
// for inner tasks. Inner tasks must be a StepDisplay task.
func (b *Builder) ParallelStep(prefix string, ignoreError bool, tasks ...*StepDisplay) *Builder {
	b.tasks = append(b.tasks, newParallelStepDisplay(prefix, ignoreError, tasks...))
	return b
}

// BuildAsStep returns a task that is wrapped by a StepDisplay. The task will print single line progress.
func (b *Builder) BuildAsStep(prefix string) *StepDisplay {
	inner := b.Build()
	return newStepDisplay(prefix, inner)
}

// GcloudCreateInstance appends a GcloudCreateInstance task to the current task collection
func (b *Builder) Deploy(user, host string) *Builder {
	b.tasks = append(b.tasks, &Deploy{
		user: user,
		host: host,
	})
	return b
}

func (b *Builder) CreateVpc(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateVpc{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateNetwork(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateNetwork{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		isPrivate:      isPrivate,
	})
	return b
}

func (b *Builder) CreateRouteTable(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateRouteTable{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		isPrivate:      isPrivate,
	})
	return b
}

func (b *Builder) CreateSecurityGroup(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo, openPortsPublic, openPortsPrivate []int) *Builder {
	b.tasks = append(b.tasks, &CreateSecurityGroup{
		pexecutor:        pexecutor,
		subClusterType:   subClusterType,
		clusterInfo:      clusterInfo,
		isPrivate:        isPrivate,
		openPortsPublic:  openPortsPublic,
		openPortsPrivate: openPortsPrivate,
	})
	return b
}

func (b *Builder) CreatePDNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.PD,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "pd",
	})
	return b
}

func (b *Builder) CreateTiDBNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.TiDB,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "tidb",
	})
	return b
}

func (b *Builder) CreateTiKVNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateTiKVNodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.TiKV,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "tikv",
	})
	return b
}

func (b *Builder) CreateTiFlashNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.TiFlash,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "tiflash",
	})
	return b
}

func (b *Builder) CreateDMMasterNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.DMMaster,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "dm-master",
	})
	return b
}

func (b *Builder) CreateDMWorkerNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.DMWorker,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "dm-worker",
	})
	return b
}

func (b *Builder) CreateTiCDCNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.TiCDC,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "ticdc",
	})
	return b
}

func (b *Builder) CreatePumpNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.Pump,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "pump",
	})
	return b
}

func (b *Builder) CreateDrainerNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.Drainer,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "drainer",
	})
	return b
}

func (b *Builder) CreateMonitorNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.Monitor,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "monitor",
	})
	return b
}

func (b *Builder) CreateGrafanaNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.Grafana,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "grafana",
	})
	return b
}

func (b *Builder) CreateAlertManagerNodes(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    &awsTopoConfigs.AlertManager,
		awsGeneralConfigs: &awsTopoConfigs.General,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     "alert-manager",
	})
	return b
}

func (b *Builder) WrapCreateEC2Nodes(pexecutor *ctxt.Executor, subClusterType string, ec2Node *spec.AwsNodeModal, awsGeneralConfig *spec.AwsTopoConfigsGeneral, clusterInfo *ClusterInfo, componentName string) *Builder {
	b.tasks = append(b.tasks, &CreateEC2Nodes{
		pexecutor:         pexecutor,
		awsTopoConfigs:    ec2Node,
		awsGeneralConfigs: awsGeneralConfig,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
		componentName:     componentName,
	})
	return b
}

func (b *Builder) CreateWorkstation(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo, wsExe *ctxt.Executor, gOpt *operator.Options) *Builder {
	b.tasks = append(b.tasks, &CreateWorkstation{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		wsExe:          wsExe,
		gOpt:           gOpt,
	})
	return b
}

func (b *Builder) CreateInternetGateway(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateInternetGateway{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) AcceptVPCPeering(pexecutor *ctxt.Executor, listComponent []string) *Builder {
	b.tasks = append(b.tasks, &AcceptVPCPeering{
		pexecutor:     pexecutor,
		listComponent: listComponent,
	})
	return b
}

func (b *Builder) DeployTiDB(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiDB{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployDM(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, tidbCloudConnInfo *spec.TiDBCloudConnInfo, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployDM{
		pexecutor:         pexecutor,
		awsWSConfigs:      awsWSConfigs,
		tidbCloudConnInfo: tidbCloudConnInfo,
		subClusterType:    subClusterType,
		clusterInfo:       clusterInfo,
	})
	return b
}

func (b *Builder) ScaleTiDB(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, awsTopoConfig *spec.AwsTopoConfigs) *Builder {
	b.tasks = append(b.tasks, &ScaleTiDB{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		awsTopoConfig:  awsTopoConfig,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateRouteTgw(pexecutor *ctxt.Executor, subClusterType string, subClusterTypes []string) *Builder {
	b.tasks = append(b.tasks, &CreateRouteTgw{
		pexecutor:       pexecutor,
		subClusterType:  subClusterType,
		subClusterTypes: subClusterTypes,
	})
	return b
}

func (b *Builder) DestroyAutoScalingGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyAutoScalingGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyLaunchTemplate(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyLaunchTemplate{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyEC(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyEC{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroySecurityGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroySecurityGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyVpcPeering(pexecutor *ctxt.Executor, listComponent []string) *Builder {
	b.tasks = append(b.tasks, &DestroyVpcPeering{
		pexecutor:     pexecutor,
		listComponent: listComponent,
	})
	return b
}

func (b *Builder) DestroyNetwork(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyNetwork{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyRouteTable(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRouteTable{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyInternetGateway(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyInternetGateway{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyVpc(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyVpc{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateDBSubnetGroup(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBSubnetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBClusterParameterGroup(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBClusterParameterGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBCluster(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBCluster{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBParameterGroup(pexecutor *ctxt.Executor, subClusterType, groupFamily string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBParameterGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		groupFamily:    groupFamily,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBInstance(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DestroyDBInstance(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBCluster(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBCluster{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBParameterGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBParameterGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBClusterParameterGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBClusterParameterGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBSubnetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBSubnetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateMS(pexecutor *ctxt.Executor, subClusterType string, awsMSConfigs *spec.AwsMSConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateMS{
		pexecutor:      pexecutor,
		awsMSConfigs:   awsMSConfigs,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployTiDBInstance(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, subClusterType, tidbVersion string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiDBInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		awsWSConfigs:   awsWSConfigs,
		tidbVersion:    tidbVersion,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) ScaleTiDBInstance(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiDBInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployKafka(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployKafka{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		awsWSConfigs:   awsWSConfigs,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployMongo(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployMongo{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		awsWSConfigs:   awsWSConfigs,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateEKSCluster(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsESConfigs *spec.AwsESTopoConfigs, subClusterType string, clusterInfo *ClusterInfo) *Builder {

	clusterInfo.cidr = awsESConfigs.General.CIDR
	clusterInfo.excludedAZ = awsESConfigs.General.ExcludedAZ
	clusterInfo.includedAZ = awsESConfigs.General.IncludedAZ
	// clusterInfo.enableNAT = awsESConfigs.General.EnableNAT
	clusterInfo.enableNAT = "true" // The nat is mandatory for EKS node group. If the NAT wants to be disable, need to set the private endpoint to acces EKS control plane

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{22, 80, 3000}, []int{}).Build()).
		Step(fmt.Sprintf("%s : Creating EKS ... ...", subClusterType), &DeployEKS{
			pexecutor:         pexecutor,
			subClusterType:    subClusterType,
			awsGeneralConfigs: &awsESConfigs.General,
			awsWSConfigs:      awsWSConfigs,
			clusterInfo:       clusterInfo,
		})

	return b
}

func (b *Builder) CreateK8SESCluster(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsESConfigs *spec.AwsESTopoConfigs, subClusterType string, clusterInfo *ClusterInfo) *Builder {

	clusterInfo.cidr = awsESConfigs.General.CIDR
	clusterInfo.excludedAZ = awsESConfigs.General.ExcludedAZ
	clusterInfo.includedAZ = awsESConfigs.General.IncludedAZ
	clusterInfo.enableNAT = awsESConfigs.General.EnableNAT

	b.Step(fmt.Sprintf("%s : Creating ES on EKS ... ...", subClusterType), &DeployK8SES{
		pexecutor:         pexecutor,
		subClusterType:    subClusterType,
		awsGeneralConfigs: &awsESConfigs.General,
		awsWSConfigs:      awsWSConfigs,
		clusterInfo:       clusterInfo,
	})

	return b
}

func (b *Builder) DestroyK8SESCluster(pexecutor *ctxt.Executor, gOpt operator.Options) *Builder {
	b.Step(" Destroying ES on EKS ... ...", &DestroyK8SES{
		pexecutor: pexecutor,
		gOpt:      gOpt,
	})

	return b
}

func (b *Builder) DestroyEKSCluster(pexecutor *ctxt.Executor, subClusterType string, gOpt operator.Options) *Builder {
	b.Step(" Destroying EKS ... ...", &DestroyEKS{pexecutor: pexecutor, gOpt: gOpt}).
		Step(fmt.Sprintf("%s : Destroying Basic resources ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())

	return b
}

func (b *Builder) DeployTiCDC(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiCDC{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) MakeDBObjects(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &MakeDBObjects{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSSourceEndpoint(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSSourceEndpoint{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSTargetEndpoint(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSTargetEndpoint{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSInstance(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSSubnetGroup(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSSubnetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSTask(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSTask{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTransitGateway(pexecutor *ctxt.Executor) *Builder {
	fmt.Printf("----------------------------- \n\n\n\n\n\n")
	b.tasks = append(b.tasks, &CreateTransitGateway{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) CreateTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateTransitGatewayVpcAttachment{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSInstance(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSInstance{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSTask(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSTask{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSEndpoints(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSEndpoints{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSSubnetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSSubnetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyTransitGatewayVpcAttachment(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGatewayVpcAttachment{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) DestroyTransitGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGateway{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) CreateBasicResource(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo, openPortsPublic, openPortsPrivate []int) *Builder {
	if isPrivate == true {
		if clusterInfo.enableNAT == "true" {
			b.Step(fmt.Sprintf("%s : Creating VPC ... ...", subClusterType), NewBuilder().CreateVpc(pexecutor, subClusterType, clusterInfo).Build()).
				Step(fmt.Sprintf("%s : Creating NAT Resource ... ...", subClusterType), NewBuilder().CreateNAT(pexecutor, subClusterType).Build()).
				Step(fmt.Sprintf("%s : Creating Route Table ... ...", subClusterType), NewBuilder().CreateRouteTable(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
				// NAT creation should be after the network preparation, otherwise the nat subnet is taken as one of the private subnets.
				Step(fmt.Sprintf("%s : Creating Network ... ... ", subClusterType), NewBuilder().CreateNetwork(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
				Step(fmt.Sprintf("%s : Attaching VPC ... ... ", subClusterType), NewBuilder().CreateTransitGatewayVpcAttachment(pexecutor, subClusterType).Build()).
				Step(fmt.Sprintf("%s : Creating Security Group ... ... ", subClusterType), NewBuilder().CreateSecurityGroup(pexecutor, subClusterType, isPrivate, clusterInfo, openPortsPublic, openPortsPrivate).Build())
		} else {
			b.Step(fmt.Sprintf("%s : Creating VPC ... ...", subClusterType), NewBuilder().CreateVpc(pexecutor, subClusterType, clusterInfo).Build()).
				Step(fmt.Sprintf("%s : Creating Route Table ... ...", subClusterType), NewBuilder().CreateRouteTable(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
				Step(fmt.Sprintf("%s : Creating Network ... ... ", subClusterType), NewBuilder().CreateNetwork(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
				Step(fmt.Sprintf("%s : Attaching VPC ... ... ", subClusterType), NewBuilder().CreateTransitGatewayVpcAttachment(pexecutor, subClusterType).Build()).
				Step(fmt.Sprintf("%s : Creating Security Group ... ... ", subClusterType), NewBuilder().CreateSecurityGroup(pexecutor, subClusterType, isPrivate, clusterInfo, openPortsPublic, openPortsPrivate).Build())
		}

	} else {
		b.Step(fmt.Sprintf("%s : Creating VPC ... ...", subClusterType), NewBuilder().CreateVpc(pexecutor, subClusterType, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating route table ... ...", subClusterType), NewBuilder().CreateRouteTable(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating network ... ...", subClusterType), NewBuilder().CreateNetwork(pexecutor, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Attaching VPC ... ... ", subClusterType), NewBuilder().CreateTransitGatewayVpcAttachment(pexecutor, subClusterType).Build()).
			Step(fmt.Sprintf("%s : Creating security group ... ...", subClusterType), NewBuilder().CreateSecurityGroup(pexecutor, subClusterType, isPrivate, clusterInfo, openPortsPublic, openPortsPrivate).Build()).
			Step(fmt.Sprintf("%s : Creating internet gateway ... ...", subClusterType), NewBuilder().CreateInternetGateway(pexecutor, subClusterType, clusterInfo).Build())
	}

	return b
}

func (b *Builder) CreateWorkstationCluster(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo, wsExe *ctxt.Executor, gOpt *operator.Options) *Builder {
	clusterInfo.cidr = awsWSConfigs.CIDR
	clusterInfo.keyFile = awsWSConfigs.KeyFile

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().CreateBasicResource(pexecutor, subClusterType, false, clusterInfo, []int{22, 80, 3000}, []int{}).Build()).
		Step(fmt.Sprintf("%s : Creating workstation ... ...", subClusterType), NewBuilder().CreateWorkstation(pexecutor, subClusterType, awsWSConfigs, clusterInfo, wsExe, gOpt).Build())

	return b
}

func (b *Builder) CreateNAT(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateNAT{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyNAT(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyNAT{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateTiDBCluster(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsTopoConfigs.General.CIDR
	clusterInfo.excludedAZ = awsTopoConfigs.General.ExcludedAZ
	clusterInfo.includedAZ = awsTopoConfigs.General.IncludedAZ
	clusterInfo.enableNAT = awsTopoConfigs.General.EnableNAT

	var parallelTasks []Task

	t1 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.PD, &awsTopoConfigs.General, clusterInfo, "pd").Build()
	parallelTasks = append(parallelTasks, t1)

	t2 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.TiDB, &awsTopoConfigs.General, clusterInfo, "tidb").
		Step(fmt.Sprintf("%s : Creating Target Group ... ...", subClusterType), NewBuilder().CreateTargetGroup(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Registering Target  ... ...", subClusterType), NewBuilder().RegisterTarget(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating Load Balancer ... ...", subClusterType), NewBuilder().CreateNLB(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating Load Balancer Listener ... ...", subClusterType), NewBuilder().CreateNLBListener(pexecutor, subClusterType, clusterInfo).Build()).Build()

	parallelTasks = append(parallelTasks, t2)

	for _, tikvGroup := range awsTopoConfigs.TiKV {
		t3 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &tikvGroup, &awsTopoConfigs.General, clusterInfo, "tikv").Build()
		parallelTasks = append(parallelTasks, t3)
	}

	t4 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.TiFlash, &awsTopoConfigs.General, clusterInfo, "tiflash").Build()
	parallelTasks = append(parallelTasks, t4)

	t5 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.DMMaster, &awsTopoConfigs.General, clusterInfo, "dm-master").Build()
	parallelTasks = append(parallelTasks, t5)

	t6 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.DMWorker, &awsTopoConfigs.General, clusterInfo, "dm-worker").Build()
	parallelTasks = append(parallelTasks, t6)

	t7 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &awsTopoConfigs.TiCDC, &awsTopoConfigs.General, clusterInfo, "ticdc").Build()
	parallelTasks = append(parallelTasks, t7)

	t8 := NewBuilder().CreateMonitorNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()
	parallelTasks = append(parallelTasks, t8)

	t9 := NewBuilder().CreateGrafanaNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()
	parallelTasks = append(parallelTasks, t9)

	t10 := NewBuilder().CreateAlertManagerNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()
	parallelTasks = append(parallelTasks, t10)

	// Step(fmt.Sprintf("%s : Creating Monitor Nodes ... ...", subClusterType), NewBuilder().CreateMonitorNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()).
	// Step(fmt.Sprintf("%s : Creating Grafana Nodes ... ...", subClusterType), NewBuilder().CreateGrafanaNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()).
	// Step(fmt.Sprintf("%s : Creating Alert Manager Nodes ... ...", subClusterType), NewBuilder().CreateAlertManagerNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()).

	//
	// 49191: thanos query port
	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 1433, 2379, 2380, 3000, 3306, 4000, 8250, 8300, 9100, 9090, 9093, 9094, 10080, 12020, 20160, 20180, 9000, 8123, 3930, 20170, 20292, 8234, 49191}).Build()).
		Step(fmt.Sprintf("%s : Creating TiDB Resource ... ...", subClusterType), NewBuilder().Parallel(false, parallelTasks...).Build())

	return b
}

func (b *Builder) CreateDMCluster(pexecutor *ctxt.Executor, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsTopoConfigs.General.CIDR
	clusterInfo.excludedAZ = awsTopoConfigs.General.ExcludedAZ
	clusterInfo.includedAZ = awsTopoConfigs.General.IncludedAZ
	clusterInfo.enableNAT = awsTopoConfigs.General.EnableNAT

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 8261, 8262, 8291, 8249, 9090, 3000, 9093, 9094}).Build()).
		Step(fmt.Sprintf("%s : Creating DM Nodes ... ...", subClusterType),
			NewBuilder().CreateDMMasterNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DM Nodes ... ...", subClusterType),
			NewBuilder().CreateDMWorkerNodes(pexecutor, subClusterType, awsTopoConfigs, clusterInfo).Build())

	return b
}

func (b *Builder) CreateKafkaCluster(pexecutor *ctxt.Executor, subClusterType string, topo *spec.AwsKafkaTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = topo.General.CIDR
	clusterInfo.excludedAZ = topo.General.ExcludedAZ
	clusterInfo.includedAZ = topo.General.IncludedAZ
	clusterInfo.enableNAT = topo.General.EnableNAT

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 9092, 2181, 8081, 8082, 8083}).Build()).
		Step(fmt.Sprintf("%s : Creating Zookeeper Nodes ... ...", subClusterType),
			NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.Zookeeper, &topo.General, clusterInfo, "zookeeper").Build()).
		Step(fmt.Sprintf("%s : Creating brokers Nodes ... ...", subClusterType),
			NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.Broker, &topo.General, clusterInfo, "broker").Build()).
		Step(fmt.Sprintf("%s : Creating schema registry Nodes ... ...", subClusterType),
			NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.SchemaRegistry, &topo.General, clusterInfo, "schemaRegistry").Build()).
		Step(fmt.Sprintf("%s : Creating rest service ... ...", subClusterType),
			NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.RestService, &topo.General, clusterInfo, "restService").Build()).
		Step(fmt.Sprintf("%s : Creating schema registry Nodes ... ...", subClusterType),
			NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.Connector, &topo.General, clusterInfo, "connector").Build())

	return b
}

func (b *Builder) CreateMongoCluster(pexecutor *ctxt.Executor, subClusterType string, topo *spec.AwsMongoTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = topo.General.CIDR
	clusterInfo.excludedAZ = topo.General.ExcludedAZ
	clusterInfo.includedAZ = topo.General.IncludedAZ
	clusterInfo.enableNAT = topo.General.EnableNAT

	var envInitTasks []Task

	t1 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.ConfigServer, &topo.General, clusterInfo, "config-server").Build()
	envInitTasks = append(envInitTasks, t1)

	t2 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &topo.Mongos, &topo.General, clusterInfo, "mongos").Build()
	envInitTasks = append(envInitTasks, t2)

	for _, _replicaSet := range topo.ReplicaSet {
		t3 := NewBuilder().WrapCreateEC2Nodes(pexecutor, subClusterType, &_replicaSet, &topo.General, clusterInfo, "replicaSet").Build()
		envInitTasks = append(envInitTasks, t3)
	}

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 27017, 27027, 27037}).Build()).
		Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().Parallel(false, envInitTasks...).Build())

	return b
}

func (b *Builder) DestroyBasicResource(pexecutor *ctxt.Executor, subClusterType string) *Builder {

	b.Step(fmt.Sprintf("%s : Destroying internet gateway ... ...", subClusterType), NewBuilder().DestroyInternetGateway(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying security group ... ...", subClusterType), NewBuilder().DestroySecurityGroup(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying nat ... ...", subClusterType), NewBuilder().DestroyNAT(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying network ... ...", subClusterType), NewBuilder().DestroyNetwork(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying route table ... ...", subClusterType), NewBuilder().DestroyRouteTable(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying VPC ... ...", subClusterType), NewBuilder().DestroyVpc(pexecutor, subClusterType).Build())

	return b
}

func (b *Builder) DestroyEC2Nodes(pexecutor *ctxt.Executor, subClusterType string) *Builder {

	b.Step(fmt.Sprintf("%s : Destroying Load balancers ... ...", subClusterType), NewBuilder().DestroyNLB(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Target Group ... ...", subClusterType), NewBuilder().DestroyTargetGroup(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Auto scaling group ... ...", subClusterType), NewBuilder().DestroyAutoScalingGroup(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Launch Templates ... ...", subClusterType), NewBuilder().DestroyLaunchTemplate(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying EC2 nodes ... ...", subClusterType), NewBuilder().DestroyEC(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic resources ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())

	return b
}

func (b *Builder) DestroyDMSService(pexecutor *ctxt.Executor, subClusterType string) *Builder {

	b.Step(fmt.Sprintf("%s : Destroying DMS Task ... ...", subClusterType), NewBuilder().DestroyDMSTask(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Instance ... ...", subClusterType), NewBuilder().DestroyDMSInstance(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Endpoints ... ...", subClusterType), NewBuilder().DestroyDMSEndpoints(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Subnet Group ... ...", subClusterType), NewBuilder().DestroyDMSSubnetGroup(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic Resource ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())

	return b
}

func (b *Builder) DestroyTransitGateways(pexecutor *ctxt.Executor) *Builder {

	b.Step(fmt.Sprintf("Destroying Transit Gateway VPC Attachment ... ..."), NewBuilder().DestroyTransitGatewayVpcAttachment(pexecutor).Build()).
		Step(fmt.Sprintf("Destroying Transit Gateway ... ..."), NewBuilder().DestroyTransitGateway(pexecutor).Build())

	return b
}

func (b *Builder) CreateSqlServer(pexecutor *ctxt.Executor, subClusterType string, awsMSConfigs *spec.AwsMSConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsMSConfigs.CIDR
	clusterInfo.keyName = awsMSConfigs.KeyName
	clusterInfo.instanceType = awsMSConfigs.InstanceType
	clusterInfo.imageId = awsMSConfigs.ImageId

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 1433}).Build()).
		Step(fmt.Sprintf("%s : Creating DB Subnet group ... ...", subClusterType), NewBuilder().CreateDBSubnetGroup(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Param Group ... ...", subClusterType), NewBuilder().CreateDBParameterGroup(pexecutor, subClusterType, awsMSConfigs.DBParameterFamilyGroup, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating MS ... ...", subClusterType), NewBuilder().CreateMS(pexecutor, subClusterType, awsMSConfigs, clusterInfo).Build())

	return b
}

func (b *Builder) DestroySqlServer(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.Step(fmt.Sprintf("%s : Destroying SQL Server ... ...", subClusterType), NewBuilder().DestroyDBInstance(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Subnet Group ... ...", subClusterType), NewBuilder().DestroyDBSubnetGroup(pexecutor, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic Resource ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())

	return b
}

func (b *Builder) CreateDMSService(pexecutor *ctxt.Executor, subClusterType string, awsDMSConfigs *spec.AwsDMSConfigs, clusterInfo *ClusterInfo) *Builder {
	if awsDMSConfigs.CIDR == "" && awsDMSConfigs.InstanceType == "" {
		return b
	}
	clusterInfo.cidr = awsDMSConfigs.CIDR
	clusterInfo.instanceType = awsDMSConfigs.InstanceType
	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType), NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{22, 1433, 2379, 2380, 3306, 4000, 8250, 8300, 9100, 10080, 20160, 20180}).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Subnet Group ... ...", subClusterType), NewBuilder().CreateDMSSubnetGroup(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Instance ... ...", subClusterType), NewBuilder().CreateDMSInstance(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Source Endpoint ... ...", subClusterType), NewBuilder().CreateDMSSourceEndpoint(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Target Endpoint ... ...", subClusterType), NewBuilder().CreateDMSTargetEndpoint(pexecutor, subClusterType, clusterInfo).Build())

	return b
}

func (b *Builder) SysbenchTiCDC(pexecutor *ctxt.Executor, identityFile string, clusterTable *[][]string) *Builder {
	b.tasks = append(b.tasks, &SysbenchTiCDC{
		pexecutor:    pexecutor,
		identityFile: identityFile, // The identity file for workstation user. To improve better.
		clusterTable: clusterTable,
	})
	return b
}

func (b *Builder) PrepareSysbenchTiCDC(pexecutor *ctxt.Executor, identityFile string, scriptParam ScriptParam) *Builder {
	b.tasks = append(b.tasks, &PrepareSysbenchTiCDC{
		pexecutor:    pexecutor,
		identityFile: identityFile, // The identity file for workstation user. To improve better.
		scriptParam:  scriptParam,
	})
	return b
}

func (b *Builder) ListVpc(pexecutor *ctxt.Executor, tableVPC *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListVpc{
		pexecutor: pexecutor,
		tableVPC:  tableVPC,
	})
	return b
}

func (b *Builder) ListAccount(pexecutor *ctxt.Executor, account *string) *Builder {
	b.tasks = append(b.tasks, &ListAccount{
		pexecutor: pexecutor,
		account:   account,
	})
	return b
}

func (b *Builder) ListNetwork(pexecutor *ctxt.Executor, tableSubnets *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListNetwork{
		pexecutor:    pexecutor,
		tableSubnets: tableSubnets,
	})
	return b
}

func (b *Builder) ListRouteTable(pexecutor *ctxt.Executor, tableRouteTables *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListRouteTable{
		pexecutor:        pexecutor,
		tableRouteTables: tableRouteTables,
	})
	return b
}

func (b *Builder) ListSecurityGroup(pexecutor *ctxt.Executor, tableSecurityGroups *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListSecurityGroup{
		pexecutor:           pexecutor,
		tableSecurityGroups: tableSecurityGroups,
	})
	return b
}

func (b *Builder) ListTransitGateway(pexecutor *ctxt.Executor, transitGateway *TransitGateway) *Builder {
	b.tasks = append(b.tasks, &ListTransitGateway{
		pexecutor:      pexecutor,
		transitGateway: transitGateway,
	})
	return b
}

func (b *Builder) ListTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, tableTransitGatewayVpcAttachments *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListTransitGatewayVpcAttachment{
		pexecutor:                         pexecutor,
		tableTransitGatewayVpcAttachments: tableTransitGatewayVpcAttachments,
	})
	return b
}

func (b *Builder) ListEC(pexecutor *ctxt.Executor, tableECs *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListEC{
		pexecutor: pexecutor,
		tableECs:  tableECs,
	})
	return b
}

func (b *Builder) ListOracle(pexecutor *ctxt.Executor, tableOracle *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListOracle{
		pexecutor:   pexecutor,
		tableOracle: tableOracle,
	})
	return b
}

func (b *Builder) ListAurora(pexecutor *ctxt.Executor, tableAurora *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListAurora{
		pexecutor:   pexecutor,
		tableAurora: tableAurora,
	})
	return b
}

func (b *Builder) ListPostgres(pexecutor *ctxt.Executor, tablePostgres *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListPostgres{
		pexecutor:     pexecutor,
		tablePostgres: tablePostgres,
	})
	return b
}

func (b *Builder) InstallOracleClient(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs) *Builder {
	b.tasks = append(b.tasks, &InstallOracleClient{
		pexecutor:    pexecutor,
		awsWSConfigs: awsWSConfigs,
	})
	return b
}

func (b *Builder) InstallTiDB(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs) *Builder {
	b.tasks = append(b.tasks, &InstallTiDB{
		pexecutor:    pexecutor,
		awsWSConfigs: awsWSConfigs,
	})
	return b
}

func (b *Builder) DeployDrainConfig(pexecutor *ctxt.Executor, awsOracleConfigs *spec.AwsOracleConfigs, awsWSConfigs *spec.AwsWSConfigs, drainerReplicate *spec.DrainerReplicate) *Builder {
	b.tasks = append(b.tasks, &DeployDrainConfig{
		pexecutor:        pexecutor,
		awsWSConfigs:     awsWSConfigs,
		awsOracleConfigs: awsOracleConfigs,
		drainerReplicate: drainerReplicate,
	})
	return b
}

func (b *Builder) ListNLB(pexecutor *ctxt.Executor, subClusterType string, nlb *elbtypes.LoadBalancer) *Builder {
	b.tasks = append(b.tasks, &ListNLB{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		nlb:            nlb,
	})
	return b
}

func (b *Builder) CreateTargetGroup(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateTargetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) RegisterTarget(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &RegisterTarget{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateNLB(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateNLB{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateNLBListener(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateNLBListener{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTiDBNLB(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.Step(fmt.Sprintf("%s : Creating Target Group ... ...", subClusterType), NewBuilder().CreateTargetGroup(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Registering Target  ... ...", subClusterType), NewBuilder().RegisterTarget(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating Load Balancer ... ...", subClusterType), NewBuilder().CreateNLB(pexecutor, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating Load Balancer Listener ... ...", subClusterType), NewBuilder().CreateNLBListener(pexecutor, subClusterType, clusterInfo).Build())

	return b
}

func (b *Builder) DestroyNLB(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyNLB{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyTargetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyTargetGroup{
		pexecutor:      pexecutor,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateCloudFormation(pexecutor *ctxt.Executor, awsCloudFormationConfigs *spec.AwsCloudFormationConfigs, cloudFormationType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateCloudFormation{
		pexecutor:                pexecutor,
		awsCloudFormationConfigs: awsCloudFormationConfigs,
		cloudFormationType:       cloudFormationType,
		clusterInfo:              clusterInfo,
	})
	return b
}

func (b *Builder) DestroyCloudFormation(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyCloudFormation{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) CreateOracle(pexecutor *ctxt.Executor, awsOracleConfigs *spec.AwsOracleConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateOracle{
		pexecutor:        pexecutor,
		awsOracleConfigs: awsOracleConfigs,
		clusterInfo:      clusterInfo,
	})
	return b
}

func (b *Builder) CreateRedshiftCluster(pexecutor *ctxt.Executor, subClusterType string, awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsRedshiftTopoConfigs.CIDR

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, true, clusterInfo, []int{}, []int{5439}).Build()).
		Step(fmt.Sprintf("%s : Creating Reshift ... ...", subClusterType), &CreateRedshiftCluster{
			BaseRedshiftCluster: BaseRedshiftCluster{pexecutor: pexecutor, awsRedshiftTopoConfigs: awsRedshiftTopoConfigs},
			clusterInfo:         clusterInfo,
		})

	return b
}

func (b *Builder) DeployRedshiftInstance(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs, wsExe *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DeployRedshiftInstance{
		BaseRedshiftCluster: BaseRedshiftCluster{pexecutor: pexecutor, awsRedshiftTopoConfigs: awsRedshiftTopoConfigs},
		awsWSConfigs:        awsWSConfigs,
		wsExe:               wsExe,
	})

	return b
}

func (b *Builder) RunCommonWS(wsExe *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &RunCommonWS{
		wsExe: wsExe,
	})
	return b
}

func (b *Builder) ListRedshiftCluster(pexecutor *ctxt.Executor, redshiftDBInfos *RedshiftDBInfos) *Builder {
	b.tasks = append(b.tasks, &ListRedshiftCluster{
		BaseRedshiftCluster: BaseRedshiftCluster{pexecutor: pexecutor, RedshiftDBInfos: redshiftDBInfos},
	})
	return b
}

func (b *Builder) DestroyRedshiftCluster(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRedshiftCluster{
		BaseRedshiftCluster: BaseRedshiftCluster{pexecutor: pexecutor},
	})

	b.Step(fmt.Sprintf("%s : Destroying Basic resources ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())
	return b
}

func (b *Builder) DestroyOracle(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyOracle{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) CreateAurora(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsAuroraConfigs *spec.AwsAuroraConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateAurora{
		pexecutor:        pexecutor,
		awsAuroraConfigs: awsAuroraConfigs,
		awsWSConfigs:     awsWSConfigs,
		clusterInfo:      clusterInfo,
	})
	return b
}

func (b *Builder) CreatePostgres(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsPostgresConfigs *spec.AwsPostgresConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreatePostgres{
		pexecutor:          pexecutor,
		awsPostgresConfigs: awsPostgresConfigs,
		awsWSConfigs:       awsWSConfigs,
		clusterInfo:        clusterInfo,
	})
	return b
}

func (b *Builder) DestroyAurora(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyAurora{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) DestroyPostgres(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyPostgres{
		pexecutor: pexecutor,
	})
	return b
}

func (b *Builder) DeployPDNS(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs) *Builder {
	b.tasks = append(b.tasks, &DeployPDNS{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DeployWS(pexecutor *ctxt.Executor, subClusterType string, awsWSConfigs *spec.AwsWSConfigs) *Builder {
	b.tasks = append(b.tasks, &DeployWS{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) TakeTimer(timer *awsutils.ExecutionTimer, exePhase string) *Builder {
	timer.Take(exePhase)

	return b
}

func (b *Builder) ListVpcPeering(pexecutor *ctxt.Executor, subClusterTypes []string, tableVpcPeeringInfo *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListVpcPeering{
		pexecutor:           pexecutor,
		subClusterTypes:     subClusterTypes,
		tableVpcPeeringInfo: tableVpcPeeringInfo,
	})
	return b
}

func (b *Builder) RunOntimeBatchInsert(pexecutor *ctxt.Executor, opt *operator.LatencyWhenBatchOptions, gOpt *operator.Options) *Builder {
	b.tasks = append(b.tasks, &RunOntimeBatchInsert{
		pexecutor: pexecutor,
		gOpt:      gOpt,
		opt:       opt,
	})
	return b
}

func (b *Builder) RunSysbench(pexecutor *ctxt.Executor, sysbenchConfigFile string, sysbenchResult *[][]string, opt *operator.LatencyWhenBatchOptions, gOpt *operator.Options, cancelCtx *context.CancelFunc) *Builder {
	b.tasks = append(b.tasks, &RunSysbench{
		pexecutor:          pexecutor,
		gOpt:               gOpt,
		opt:                opt,
		sysbenchConfigFile: sysbenchConfigFile,
		sysbenchResult:     sysbenchResult,
		cancelCtx:          cancelCtx,
	})
	return b
}

func (b *Builder) ListAwsEC2(pexecutor *ctxt.Executor, tableEC2 *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListAllAwsEC2{
		pexecutor: pexecutor,
		tableEC2:  tableEC2,
	})
	return b
}

func (b *Builder) CreateTiDBCloud(tidbCloud *spec.TiDBCloud) *Builder {
	b.tasks = append(b.tasks, &CreateTiDBCloud{
		tidbCloud: tidbCloud,
	})
	return b
}

func (b *Builder) ListTiDBCloud(projectID uint64, status, clusterType string, tableClusters *[][]string, tableNodes *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListTiDBCloud{
		projectID:     projectID,
		status:        status,
		clusterType:   clusterType,
		tableClusters: tableClusters,
		tableNodes:    tableNodes,
	})
	return b
}

func (b *Builder) DeployThanos(opt *operator.ThanosS3Config, gOpt *operator.Options) *Builder {
	b.tasks = append(b.tasks, &DeployThanos{
		opt:  opt,
		gOpt: gOpt,
	})
	return b
}
