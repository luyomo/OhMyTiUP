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

	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"

	"github.com/luyomo/tisample/pkg/crypto"
	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/proxy"
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

func (b *Builder) CreateVpc(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateVpc{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateNetwork(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateNetwork{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		isPrivate:      isPrivate,
	})
	return b
}

func (b *Builder) CreateRouteTable(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateRouteTable{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		isPrivate:      isPrivate,
	})
	return b
}

func (b *Builder) CreateSecurityGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateSecurityGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
		isPrivate:      isPrivate,
	})
	return b
}

func (b *Builder) CreatePDNodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreatePDNodes{
		pexecutor:      pexecutor,
		awsTopoConfigs: awsTopoConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTiDBNodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateTiDBNodes{
		pexecutor:      pexecutor,
		awsTopoConfigs: awsTopoConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTiKVNodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateTiKVNodes{
		pexecutor:      pexecutor,
		awsTopoConfigs: awsTopoConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMNodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMNodes{
		pexecutor:      pexecutor,
		awsTopoConfigs: awsTopoConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTiCDCNodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateTiCDCNodes{
		pexecutor:      pexecutor,
		awsTopoConfigs: awsTopoConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateWorkstation(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateWorkstation{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateInternetGateway(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateInternetGateway{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) AcceptVPCPeering(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {
	b.tasks = append(b.tasks, &AcceptVPCPeering{
		pexecutor:   pexecutor,
		clusterName: clusterName,
		clusterType: clusterType,
	})
	return b
}

func (b *Builder) DeployTiDB(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiDB{
		pexecutor:      pexecutor,
		awsWSConfigs:   awsWSConfigs,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateRouteTgw(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, subClusterTypes []string) *Builder {
	b.tasks = append(b.tasks, &CreateRouteTgw{
		pexecutor:       pexecutor,
		clusterName:     clusterName,
		clusterType:     clusterType,
		subClusterType:  subClusterType,
		subClusterTypes: subClusterTypes,
	})
	return b
}

func (b *Builder) DestroyEC(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyEC{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroySecurityGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroySecurityGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyVpcPeering(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyVpcPeering{
		pexecutor:   pexecutor,
		clusterName: clusterName,
		clusterType: clusterType,
	})
	return b
}

func (b *Builder) DestroyNetwork(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyNetwork{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyRouteTable(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRouteTable{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyInternetGateway(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyInternetGateway{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyVpc(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyVpc{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateDBSubnetGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBSubnetGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBClusterParameterGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBClusterParameterGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBCluster(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBCluster{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBParameterGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType, groupFamily string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBParameterGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		groupFamily:    groupFamily,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDBInstance(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDBInstance{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DestroyDBInstance(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBInstance{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBCluster(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBCluster{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBParameterGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBParameterGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBClusterParameterGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBClusterParameterGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDBSubnetGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDBSubnetGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) CreateMS(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsMSConfigs *spec.AwsMSConfigs, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateMS{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		awsMSConfigs:   awsMSConfigs,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployTiDBInstance(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiDBInstance{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) DeployTiCDC(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &DeployTiCDC{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) MakeDBObjects(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &MakeDBObjects{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSSourceEndpoint(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSSourceEndpoint{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSTargetEndpoint(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSTargetEndpoint{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSInstance(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSInstance{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSSubnetGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSSubnetGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateDMSTask(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateDMSTask{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
		clusterInfo:    clusterInfo,
	})
	return b
}

func (b *Builder) CreateTransitGateway(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateTransitGateway{
		pexecutor:   pexecutor,
		clusterName: clusterName,
		clusterType: clusterType,
	})
	return b
}

func (b *Builder) CreateTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateTransitGatewayVpcAttachment{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSInstance(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSInstance{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSTask(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSTask{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSEndpoints(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSEndpoints{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyDMSSubnetGroup(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyDMSSubnetGroup{
		pexecutor:      pexecutor,
		clusterName:    clusterName,
		clusterType:    clusterType,
		subClusterType: subClusterType,
	})
	return b
}

func (b *Builder) DestroyTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGatewayVpcAttachment{
		pexecutor:   pexecutor,
		clusterName: clusterName,
		clusterType: clusterType,
	})
	return b
}

func (b *Builder) DestroyTransitGateway(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGateway{
		pexecutor:   pexecutor,
		clusterName: clusterName,
		clusterType: clusterType,
	})
	return b
}

func (b *Builder) CreateBasicResource(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)
	if isPrivate == true {
		b.Step(fmt.Sprintf("%s : Creating VPC ... ...", titleMsg), NewBuilder().CreateVpc(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating Route Table ... ...", titleMsg), NewBuilder().CreateRouteTable(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating Network ... ... ", titleMsg), NewBuilder().CreateNetwork(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating Security Group ... ... ", titleMsg), NewBuilder().CreateSecurityGroup(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build())
	} else {
		b.Step(fmt.Sprintf("%s : Creating VPC ... ...", titleMsg), NewBuilder().CreateVpc(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating route table ... ...", titleMsg), NewBuilder().CreateRouteTable(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating network ... ...", titleMsg), NewBuilder().CreateNetwork(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating security group ... ...", titleMsg), NewBuilder().CreateSecurityGroup(pexecutor, clusterName, clusterType, subClusterType, isPrivate, clusterInfo).Build()).
			Step(fmt.Sprintf("%s : Creating internet gateway ... ...", titleMsg), NewBuilder().CreateInternetGateway(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build())
	}

	return b
}

func (b *Builder) CreateWorkstationCluster(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsWSConfigs.CIDR
	clusterInfo.keyFile = awsWSConfigs.KeyFile

	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", titleMsg), NewBuilder().CreateBasicResource(pexecutor, clusterName, clusterType, subClusterType, false, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating workstation ... ...", titleMsg), NewBuilder().CreateWorkstation(pexecutor, clusterName, clusterType, subClusterType, awsWSConfigs, clusterInfo).Build())

	return b
}

func (b *Builder) CreateTiDBCluster(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsTopoConfigs *spec.AwsTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsTopoConfigs.General.CIDR

	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)
	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", titleMsg), NewBuilder().CreateBasicResource(pexecutor, clusterName, clusterType, subClusterType, true, clusterInfo).Build()).
		//		CreateWorkstation(user, host, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).
		Step(fmt.Sprintf("%s : Creating PD Nodes ... ...", titleMsg), NewBuilder().CreatePDNodes(pexecutor, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating TiDB Nodes ... ...", titleMsg), NewBuilder().CreateTiDBNodes(pexecutor, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating TiKV Nodes ... ...", titleMsg), NewBuilder().CreateTiKVNodes(pexecutor, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DM Nodes ... ...", titleMsg), NewBuilder().CreateDMNodes(pexecutor, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating TiCDC Nodes ... ...", titleMsg), NewBuilder().CreateTiCDCNodes(pexecutor, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo).Build())
		//		DeployTiDB(user, host, clusterName, clusterType, subClusterType, awsTopoConfigs, clusterInfo)

	return b
}

func (b *Builder) CreateAurora(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsAuroraConfigs *spec.AwsAuroraConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsAuroraConfigs.CIDR

	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", titleMsg), NewBuilder().CreateBasicResource(pexecutor, clusterName, clusterType, subClusterType, true, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Subnet group ... ...", titleMsg), NewBuilder().CreateDBSubnetGroup(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Cluster parameter Group ... ...", titleMsg), NewBuilder().CreateDBClusterParameterGroup(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Cluster ... ...", titleMsg), NewBuilder().CreateDBCluster(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Param Group ... ...", titleMsg), NewBuilder().CreateDBParameterGroup(pexecutor, clusterName, clusterType, subClusterType, awsAuroraConfigs.DBParameterFamilyGroup, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Instance ... ...", titleMsg), NewBuilder().CreateDBInstance(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build())

	return b
}

func (b *Builder) DestroyBasicResource(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)
	b.Step(fmt.Sprintf("%s : Destroying internet gateway ... ...", titleMsg), NewBuilder().DestroyInternetGateway(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying security group ... ...", titleMsg), NewBuilder().DestroySecurityGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying network ... ...", titleMsg), NewBuilder().DestroyNetwork(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying route table ... ...", titleMsg), NewBuilder().DestroyRouteTable(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying VPC ... ...", titleMsg), NewBuilder().DestroyVpc(pexecutor, clusterName, clusterType, subClusterType).Build())

	return b
}

func (b *Builder) DestroyEC2Nodes(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Destroying VPC ... ...", titleMsg), NewBuilder().DestroyEC(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying VPC ... ...", titleMsg), NewBuilder().DestroyBasicResource(pexecutor, clusterName, clusterType, subClusterType).Build())

	return b
}

func (b *Builder) DestroyDMSService(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Destroying DMS Task ... ...", titleMsg), NewBuilder().DestroyDMSTask(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Instance ... ...", titleMsg), NewBuilder().DestroyDMSInstance(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Endpoints ... ...", titleMsg), NewBuilder().DestroyDMSEndpoints(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DMS Subnet Group ... ...", titleMsg), NewBuilder().DestroyDMSSubnetGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic Resource ... ...", titleMsg), NewBuilder().DestroyBasicResource(pexecutor, clusterName, clusterType, subClusterType).Build())

	return b
}

func (b *Builder) DestroyTransitGateways(pexecutor *ctxt.Executor, clusterName, clusterType string) *Builder {

	titleMsg := fmt.Sprintf(" %s - %s  ", clusterName, clusterType)

	b.Step(fmt.Sprintf("%s : Destroying Transit Gateway VPC Attachment ... ...", titleMsg), NewBuilder().DestroyTransitGatewayVpcAttachment(pexecutor, clusterName, clusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Transit Gateway ... ...", titleMsg), NewBuilder().DestroyTransitGateway(pexecutor, clusterName, clusterType).Build())

	return b
}

func (b *Builder) DestroyAurora(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Destroying DBN Instance ... ...", titleMsg), NewBuilder().DestroyDBInstance(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Cluster ... ...", titleMsg), NewBuilder().DestroyDBCluster(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Paramter Group ... ...", titleMsg), NewBuilder().DestroyDBParameterGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Cluster Parameter Group  ... ...", titleMsg), NewBuilder().DestroyDBClusterParameterGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Subnet Group ... ...", titleMsg), NewBuilder().DestroyDBSubnetGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic Resource ... ...", titleMsg), NewBuilder().DestroyBasicResource(pexecutor, clusterName, clusterType, subClusterType).Build())

	return b
}

func (b *Builder) CreateSqlServer(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsMSConfigs *spec.AwsMSConfigs, clusterInfo *ClusterInfo) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	clusterInfo.cidr = awsMSConfigs.CIDR
	clusterInfo.keyName = awsMSConfigs.KeyName
	clusterInfo.instanceType = awsMSConfigs.InstanceType
	clusterInfo.imageId = awsMSConfigs.ImageId

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", titleMsg), NewBuilder().CreateBasicResource(pexecutor, clusterName, clusterType, subClusterType, true, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Subnet group ... ...", titleMsg), NewBuilder().CreateDBSubnetGroup(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DB Param Group ... ...", titleMsg), NewBuilder().CreateDBParameterGroup(pexecutor, clusterName, clusterType, subClusterType, awsMSConfigs.DBParameterFamilyGroup, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating MS ... ...", titleMsg), NewBuilder().CreateMS(pexecutor, clusterName, clusterType, subClusterType, awsMSConfigs, clusterInfo).Build())

	return b
}

func (b *Builder) DestroySqlServer(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, clusterInfo *ClusterInfo) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)

	b.Step(fmt.Sprintf("%s : Destroying SQL Server ... ...", titleMsg), NewBuilder().DestroyDBInstance(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying DB Subnet Group ... ...", titleMsg), NewBuilder().DestroyDBSubnetGroup(pexecutor, clusterName, clusterType, subClusterType).Build()).
		Step(fmt.Sprintf("%s : Destroying Basic Resource ... ...", titleMsg), NewBuilder().DestroyBasicResource(pexecutor, clusterName, clusterType, subClusterType).Build())

	return b
}

func (b *Builder) CreateDMSService(pexecutor *ctxt.Executor, clusterName, clusterType, subClusterType string, awsDMSConfigs *spec.AwsDMSConfigs, clusterInfo *ClusterInfo) *Builder {
	titleMsg := fmt.Sprintf(" %s - %s - %s ", clusterName, clusterType, subClusterType)
	// Step(fmt.Sprintf("%s : Destroying VPC ... ...", titleMsg), NewBuilder()..Build()  ).

	clusterInfo.cidr = awsDMSConfigs.CIDR
	clusterInfo.instanceType = awsDMSConfigs.InstanceType
	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", titleMsg), NewBuilder().CreateBasicResource(pexecutor, clusterName, clusterType, subClusterType, true, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Subnet Group ... ...", titleMsg), NewBuilder().CreateDMSSubnetGroup(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Instance ... ...", titleMsg), NewBuilder().CreateDMSInstance(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Source Endpoint ... ...", titleMsg), NewBuilder().CreateDMSSourceEndpoint(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build()).
		Step(fmt.Sprintf("%s : Creating DMS Target Endpoint ... ...", titleMsg), NewBuilder().CreateDMSTargetEndpoint(pexecutor, clusterName, clusterType, subClusterType, clusterInfo).Build())

	return b
}

func (b *Builder) SysbenchTiCDC(pexecutor *ctxt.Executor, identityFile, clusterName, clusterType string, clusterTable *[][]string) *Builder {
	b.tasks = append(b.tasks, &SysbenchTiCDC{
		pexecutor:    pexecutor,
		identityFile: identityFile, // The identity file for workstation user. To improve better.
		clusterName:  clusterName,
		clusterType:  clusterType,
		clusterTable: clusterTable,
	})
	return b
}

func (b *Builder) PrepareSysbenchTiCDC(pexecutor *ctxt.Executor, identityFile, clusterName, clusterType string, scriptParam ScriptParam) *Builder {
	b.tasks = append(b.tasks, &PrepareSysbenchTiCDC{
		pexecutor:    pexecutor,
		identityFile: identityFile, // The identity file for workstation user. To improve better.
		clusterName:  clusterName,
		clusterType:  clusterType,
		scriptParam:  scriptParam,
	})
	return b
}
