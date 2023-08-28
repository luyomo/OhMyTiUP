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
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/executor"

	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

func (b *Builder) DeployTiDB(subClusterType string, awsWSConfigs *spec.AwsWSConfigs, tidbVersion string, enableAuditLog bool, workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DeployTiDB{
		awsWSConfigs:   awsWSConfigs,
		subClusterType: subClusterType,
		enableAuditLog: enableAuditLog,
		tidbVersion:    tidbVersion,
		workstation:    workstation,
	})
	return b
}

type DeployTiDB struct {
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string

	tidbVersion    string
	enableAuditLog bool
	workstation    *ws.Workstation
}

// Execute implements the Task interface
func (c *DeployTiDB) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if err := c.workstation.InstallPackages(&[]string{"mariadb-client"}); err != nil {
		return err
	}

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType
	mapArgs["subClusterType"] = c.subClusterType

	ec2api, err := ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}
	instances, err := ec2api.ExtractEC2Instances()
	if err != nil {
		return err
	}
	tidbConfig := make(map[string]interface{})
	tidbConfig["Servers"] = instances
	if c.enableAuditLog == true {
		tidbConfig["AuditLog"] = "whitelist-1,audit-1"
		tidbConfig["PluginDir"] = "/home/admin/tidb/tidb-deploy/tidb-4000/plugin"
	}

	if c.awsWSConfigs.EnableMonitoring == "enabled" {
		var ipList []interface{}
		ipList = append(ipList, c.workstation.GetIPAddr())
		if len((*instances)["Grafana"]) == 0 {
			(*instances)["Grafana"] = ipList
		}

		if len((*instances)["Monitor"]) == 0 {
			(*instances)["Monitor"] = ipList
		}
	}

	wsExe, err := c.workstation.GetExecutor()
	if err != nil {
		return err
	}

	if err := (*wsExe).TransferTemplate(ctx, "templates/config/cdc-task.toml.tpl", "/opt/tidb/cdc-task.toml", "0644", nil, true, 0); err != nil {
		return err
	}

	if err := (*wsExe).TransferTemplate(ctx, "templates/config/tidb-cluster.yml.tpl", "/opt/tidb/tidb-cluster.yml", "0644", tidbConfig, true, 0); err != nil {
		return err
	}

	sqlFiles := []string{"ontime_ms.ddl", "ontime_mysql.ddl", "ontime_tidb.ddl"}
	for _, sqlFile := range sqlFiles {
		err = (*wsExe).TransferTemplate(ctx, fmt.Sprintf("templates/sql/%s", sqlFile), fmt.Sprintf("/opt/tidb/sql/%s", sqlFile), "0644", nil, true, 0)
		if err != nil {
			return err
		}
	}

	for _, tikvNode := range (*instances)["TiKV"] {
		if err := c.workstation.FormatDisk(tikvNode.(map[string]interface{})["IPAddress"].(string), "/home/admin/tidb"); err != nil {
			return err
		}
	}

	for _, tiflashNode := range (*instances)["TiFlash"] {
		if err := c.workstation.FormatDisk(tiflashNode.(string), "/home/admin/tidb"); err != nil {
			return err
		}
	}

	if c.enableAuditLog == true {
		if err := c.workstation.InstallEnterpriseTiup(c.tidbVersion); err != nil {
			return err
		}
	} else {
		if err := c.workstation.InstallTiup(); err != nil {
			return err
		}
	}

	return nil

}

// Rollback implements the Task interface
func (c *DeployTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiDB) String() string {
	return fmt.Sprintf("Echo: Deploying TiDB")
}

// Deploy thanos

type DeployThanos struct {
	opt  *operator.ThanosS3Config
	gOpt *operator.Options
}

func (c *DeployThanos) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapFilters := make(map[string]string)
	mapFilters["Name"] = clusterName
	mapFilters["Cluster"] = clusterType
	mapFilters["Component"] = "workstation"

	var _wsPublicIP *string
	_funcGetWS := func(_instance types.Instance) error {
		_wsPublicIP = _instance.PublicIpAddress
		return nil
	}
	if err := FetchInstances(ctx, &mapFilters, &_funcGetWS); err != nil {
		return err
	}

	mapFilters["Component"] = "monitor"

	var tasks []Task

	var _storeServers []string
	_funcGetStores := func(_instance types.Instance) error {
		_storeServers = append(_storeServers, *(_instance.PrivateIpAddress))
		return nil
	}

	if err := FetchInstances(ctx, &mapFilters, &_funcGetStores); err != nil {
		return err
	}

	_func := func(_instance types.Instance) error {
		tasks = append(tasks, &InstallThanos{
			TargetServer: _instance.PrivateIpAddress,
			ProxyServer:  _wsPublicIP,
			User:         (*c.gOpt).SSHUser,
			KeyFile:      (*c.gOpt).IdentityFile,
			opt:          c.opt,
			StoreServers: &_storeServers,
		})

		return nil
	}

	if err := FetchInstances(ctx, &mapFilters, &_func); err != nil {
		return err
	}
	// fmt.Printf("The ips are <%#v> \n\n\n\n", monitorIPs)

	parallelExe := Parallel{ignoreError: false, inner: tasks}
	if err := parallelExe.Execute(ctx); err != nil {
		return err
	}

	// for _, _monitorServer := range *monitorIPs {

	return nil
}

// Rollback implements the Task interface
func (c *DeployThanos) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployThanos) String() string {
	return fmt.Sprintf("Echo: Deploying Thanos")
}

type InstallThanos struct {
	TargetServer *string
	ProxyServer  *string
	User         string
	KeyFile      string

	opt          *operator.ThanosS3Config
	StoreServers *[]string
}

// #####################################################################
// # template render and update config
// #####################################################################
func (c *InstallThanos) Execute(ctx context.Context) error {

	// #####################################################################
	// # thanos install
	// #####################################################################

	// 2. Install thanos
	// 2.1 Download binary to workstation.
	// 2.2 Move the binary to monitoring server

	_promExe, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: *(c.TargetServer), User: c.User, KeyFile: c.KeyFile, Proxy: &executor.SSHConfig{Host: *(c.ProxyServer), User: c.User, Port: 22, KeyFile: c.KeyFile}}, []string{})

	if err != nil {
		return err
	}
	_, _, err = _promExe.Execute(ctx, "apt-get -y update", true)
	if err != nil {
		return err
	}

	// Install thanos
	stdout, _, err := _promExe.Execute(ctx, "which /opt/thanos-0.28.0.linux-amd64/thanos | wc -l", true)
	if err != nil {
		fmt.Printf("Error: %#v \n\n\n", err)
		return err
	}

	_cnt, err := strconv.Atoi(strings.Replace(string(stdout), "\n", "", -1))
	if err != nil {
		return err
	}

	if _cnt == 0 {

		_, _, err = _promExe.Execute(ctx, "rm -f /tmp/thanos-0.28.0.linux-amd64.tar.gz", true)
		if err != nil {
			return err
		}

		_, _, err = _promExe.Execute(ctx, "wget https://github.com/thanos-io/thanos/releases/download/v0.28.0/thanos-0.28.0.linux-amd64.tar.gz -P /tmp", false)
		if err != nil {
			return err
		}

		_, _, err = _promExe.Execute(ctx, "tar xvf /tmp/thanos-0.28.0.linux-amd64.tar.gz --directory /opt", true)
		if err != nil {
			return err
		}
	}

	_, _, err = _promExe.Execute(ctx, "mkdir -p /opt/thanos/etc", true)
	if err != nil {
		return err
	}

	// #####################################################################
	// # template render and update config
	// #####################################################################

	// Render s3 config file
	if err = _promExe.TransferTemplate(ctx, "templates/config/thanos/s3.config.yaml.tpl", "/opt/thanos/etc/s3.config.yaml", "0644", *c.opt, true, 20); err != nil {
		fmt.Printf("Error: %#v \n\n\n", err)
		return err
	}

	// Render thanos-sidecar
	if err = _promExe.TransferTemplate(ctx, "templates/config/thanos/thanos-sidecar.service.tpl", "/etc/systemd/system/thanos-sidecar.service", "0644", *c.opt, true, 20); err != nil {
		fmt.Printf("Error: %#v \n\n\n", err)
		return err
	}

	// Added enable-lifecycle/min-block-duration/max-block-duration to run_prometheus
	//   Check whether min-block-duration has existed in the file
	stdout, _, err = _promExe.Execute(ctx, "grep 'storage.tsdb.min-block-duration' /home/admin/tidb/tidb-deploy/prometheus-9090/scripts/run_prometheus.sh  | wc -l", true)
	if err != nil {
		fmt.Printf("Error: %#v \n\n\n", err)
		return err
	}

	_cnt, err = strconv.Atoi(strings.Replace(string(stdout), "\n", "", -1))
	if err != nil {
		return err
	}

	if _cnt == 0 {
		if _, _, err = _promExe.Execute(ctx, "sed -i '\\$s/$/ \\\\\\\\/' /home/admin/tidb/tidb-deploy/prometheus-9090/scripts/run_prometheus.sh", true); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

		if _, _, err = _promExe.Execute(ctx, "echo '    --web.enable-lifecycle \\' >> /home/admin/tidb/tidb-deploy/prometheus-9090/scripts/run_prometheus.sh", true); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

		if _, _, err = _promExe.Execute(ctx, "echo '    --storage.tsdb.min-block-duration=\\\"2h\\\" \\' >> /home/admin/tidb/tidb-deploy/prometheus-9090/scripts/run_prometheus.sh", true); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

		if _, _, err = _promExe.Execute(ctx, "echo '    --storage.tsdb.max-block-duration=\\\"2h\\\"' >> /home/admin/tidb/tidb-deploy/prometheus-9090/scripts/run_prometheus.sh", true); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

	}

	// Render thanos store template
	if err = _promExe.TransferTemplate(ctx, "templates/config/thanos/thanos-store.service.tpl", "/etc/systemd/system/thanos-store.service", "0644", *c.opt, true, 20); err != nil {
		return err
	}

	// Render thanos query template
	if err = _promExe.TransferTemplate(ctx, "templates/config/thanos/thanos-query.service.tpl", "/etc/systemd/system/thanos-query.service", "0644", c, true, 20); err != nil {
		return err
	}

	// #####################################################################
	// # Restart prometheus / thanos-sidecar/ thanos-store / thanos-query service
	// #####################################################################

	if _, _, err = _promExe.Execute(ctx, "systemctl daemon-reload", true); err != nil {
		return err
	}

	if _, _, err = _promExe.Execute(ctx, "systemctl restart prometheus-9090", true); err != nil {
		return err
	}

	if _, _, err = _promExe.Execute(ctx, "systemctl restart thanos-sidecar", true); err != nil {
		return err
	}

	if _, _, err = _promExe.Execute(ctx, "systemctl restart thanos-store", true); err != nil {
		return err
	}

	if _, _, err = _promExe.Execute(ctx, "systemctl restart thanos-query", true); err != nil {
		return err
	}

	return nil
}

func (c *InstallThanos) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

func (c *InstallThanos) String() string {
	return fmt.Sprintf("Echo: Deploying thanos on target server")
}
