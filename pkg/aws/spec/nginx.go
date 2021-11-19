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

package spec

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luyomo/tisample/pkg/aws/api"
	"github.com/luyomo/tisample/pkg/aws/ctxt"
	"github.com/luyomo/tisample/pkg/aws/template/scripts"
	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/meta"
	//"github.com/luyomo/tisample/pkg/utils"
	"github.com/pingcap/errors"
)

// NginxSpec represents the PD topology specification in topology.yaml
type NginxSpec struct {
	Host           string `yaml:"host"`
	ListenHost     string `yaml:"listen_host,omitempty"`
	SSHPort        int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported       bool   `yaml:"imported,omitempty"`
	Patched        bool   `yaml:"patched,omitempty"`
	IgnoreExporter bool   `yaml:"ignore_exporter,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name"`
	ClientPort      int                    `yaml:"client_port" default:"18080"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *NginxSpec) Status(tlsCfg *tls.Config, _ ...string) string {
	addr := fmt.Sprintf("%s:%d", s.Host, s.ClientPort)
	pc := api.NewPDClient([]string{addr}, statusQueryTimeout, tlsCfg)

	// check health
	err := pc.CheckHealth()
	if err != nil {
		return "Down"
	}

	// find leader node
	leader, err := pc.GetLeader()
	if err != nil {
		return "ERR"
	}
	res := "Up"
	if s.Name == leader.Name {
		res += "|L"
	}
	return res
}

// Role returns the component role of the instance
func (s *NginxSpec) Role() string {
	return ComponentNginx
}

// SSH returns the host and SSH port of the instance
func (s *NginxSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *NginxSpec) GetMainPort() int {
	return s.ClientPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *NginxSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *NginxSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// NginxComponent represents PD component.
type NginxComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *NginxComponent) Name() string {
	return ComponentNginx
}

// Role implements Component interface.
func (c *NginxComponent) Role() string {
	return ComponentNginx
}

// Instances implements Component interface.
func (c *NginxComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.NginxServers))
	for _, s := range c.Topology.NginxServers {
		s := s
		ins = append(ins, &NginxInstance{
			Name: s.Name,
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ListenHost:   s.ListenHost,
				Port:         s.ClientPort,
				SSHP:         s.SSHPort,

				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: s.Status,
				UptimeFn: func(tlsCfg *tls.Config) time.Duration {
					return UptimeByHost(s.Host, s.ClientPort, tlsCfg)
				},
			},
			topo: c.Topology,
		})
	}
	return ins
}

// PDInstance represent the PD instance
type NginxInstance struct {
	Name string
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *NginxInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(ctx, e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	enableTLS := topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(*PDSpec)
	cfg := scripts.
		NewPDScript(spec.Name, i.GetHost(), paths.Deploy, paths.Data[0], paths.Log).
		WithClientPort(spec.ClientPort).
		WithPeerPort(spec.PeerPort).
		AppendEndpoints(topo.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())

	if enableTLS {
		cfg = cfg.WithScheme("https")
	}
	cfg = cfg.WithAdvertiseClientAddr(spec.AdvertiseClientAddr).
		WithAdvertisePeerAddr(spec.AdvertisePeerAddr)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0); err != nil {
		return err
	}
	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.PD
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			AnsibleImportedConfigPath,
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		globalConfig, err = mergeImported(importConfig, globalConfig)
		if err != nil {
			return err
		}
	}

	// set TLS configs
	if enableTLS {
		if spec.Config == nil {
			spec.Config = make(map[string]interface{})
		}
		spec.Config["security.cacert-path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		spec.Config["security.cert-path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		spec.Config["security.key-path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	}

	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths, nil)
}

// ScaleConfig deploy temporary config on scaling
func (i *NginxInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	// We need pd.toml here, but we don't need to check it
	if err := i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths); err != nil &&
		errors.Cause(err) != ErrorCheckConfig {
		return err
	}

	cluster := mustBeClusterTopo(topo)

	spec := i.InstanceSpec.(*PDSpec)
	cfg0 := scripts.NewPDScript(
		i.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPeerPort(spec.PeerPort).
		WithNumaNode(spec.NumaNode).
		WithClientPort(spec.ClientPort).
		AppendEndpoints(cluster.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		cfg0 = cfg0.WithScheme("https")
	}
	cfg0 = cfg0.WithAdvertiseClientAddr(spec.AdvertiseClientAddr).
		WithAdvertisePeerAddr(spec.AdvertisePeerAddr)
	cfg := scripts.NewPDScaleScript(cfg0)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	log.Infof("script path: %s", fp)
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0); err != nil {
		return err
	}
	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}
	return nil
}
