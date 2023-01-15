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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"
	// "time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/embed"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"go.uber.org/zap"
)

type DeployTiDB struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

type TplTiKVData struct {
	IPAddress string
	Labels    []map[string]string
}

type TplTiupData struct {
	PD           []string
	TiDB         []string
	TiFlash      []string
	TiKV         []TplTiKVData
	TiCDC        []string
	DM           []string
	Monitor      []string
	Grafana      []string
	AlertManager []string
	Pump         []string
	Drainer      []string

	Labels []string
}

func (t TplTiupData) String() string {
	return fmt.Sprintf("PD: %s  |  TiDB: %s | TiFlash: %s   |  TiCDC: %s  |  DM: %s  |  Pump:%s  | Drainer: %s  | Monitor:%s | Grafana: %s | AlertManager: %s", strings.Join(t.PD, ","), strings.Join(t.TiDB, ","), strings.Join(t.TiFlash, ","), strings.Join(t.TiCDC, ","), strings.Join(t.DM, ","), strings.Join(t.Pump, ","), strings.Join(t.Drainer, ","), strings.Join(t.Monitor, ","), strings.Join(t.Grafana, ","), strings.Join(t.AlertManager, ","))
}

// Execute implements the Task interface
func (c *DeployTiDB) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	fmt.Printf("--------------------- Starting to deploy tidb \n\n\n\n\n")

	wsInfo, err := getWorkstation(*c.pexecutor, ctx, clusterName, clusterType)
	if err != nil {
		return err
	}

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 2. Get all the nodes from tag definition
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	var tplData TplTiupData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "pd" {
					tplData.PD = append(tplData.PD, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "tidb" {
					tplData.TiDB = append(tplData.TiDB, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "tiflash" {
					tplData.TiFlash = append(tplData.TiFlash, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "tikv" {
					// tplData.TiKV = append(tplData.TiKV, instance.PrivateIpAddress)

					tplTiKVData := TplTiKVData{IPAddress: instance.PrivateIpAddress}

					for _, tag := range instance.Tags {

						if strings.Contains(tag["Key"], "label:") {
							tagKey := strings.Replace(tag["Key"], "label:", "", 1)
							tplTiKVData.Labels = append(tplTiKVData.Labels, map[string]string{tagKey: tag["Value"]})

							existsInArray := false
							for _, element := range tplData.Labels {
								if element == tagKey {
									existsInArray = true
								}
							}
							if existsInArray == false {
								tplData.Labels = append(tplData.Labels, tagKey)
							}
						}
					}
					tplData.TiKV = append(tplData.TiKV, tplTiKVData)

				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" {
					tplData.TiCDC = append(tplData.TiCDC, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "dm" {
					tplData.DM = append(tplData.DM, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "pump" {
					tplData.Pump = append(tplData.Pump, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "drainer" {
					tplData.Drainer = append(tplData.Drainer, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "monitor" {
					tplData.Monitor = append(tplData.Monitor, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "grafana" {
					tplData.Grafana = append(tplData.Grafana, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "alert-manager" {
					tplData.AlertManager = append(tplData.AlertManager, instance.PrivateIpAddress)

				}
			}
		}
	}

	zap.L().Debug("AWS WS Config:", zap.String("Monitoring", c.awsWSConfigs.EnableMonitoring))
	zap.L().Debug("TiDB components:", zap.String("Componens", tplData.String()))
	if c.awsWSConfigs.EnableMonitoring == "enabled" {
		workstation, err := getWorkstation(*c.pexecutor, ctx, clusterName, clusterType)
		if err != nil {
			return err
		}

		if len(tplData.Grafana) == 0 {
			tplData.Grafana = append(tplData.Grafana, workstation.PrivateIpAddress)
		}

		if len(tplData.Monitor) == 0 {
			tplData.Monitor = append(tplData.Monitor, workstation.PrivateIpAddress)
		}

		if len(tplData.AlertManager) == 0 {
			tplData.AlertManager = append(tplData.AlertManager, workstation.PrivateIpAddress)
		}
	}
	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplData.String()))

	// 3. Make all the necessary folders
	if _, _, err := (*workstation).Execute(ctx, `mkdir -p /opt/tidb/sql`, true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`chown -R %s:%s /opt/tidb`, c.awsWSConfigs.UserName, c.awsWSConfigs.UserName), true); err != nil {
		return err
	}

	// 4. Deploy all tidb templates
	configFiles := []string{"cdc-task.toml", "tidb-cluster.yml"}
	for _, configFile := range configFiles {
		fdFile, err := os.Create(fmt.Sprintf("/tmp/%s", configFile))
		if err != nil {
			return err
		}
		defer fdFile.Close()

		fp := path.Join("templates", "config", fmt.Sprintf("%s.tpl", configFile))
		tpl, err := embed.ReadTemplate(fp)
		if err != nil {
			return err
		}

		tmpl, err := template.New("test").Parse(string(tpl))
		if err != nil {
			return err
		}

		if err := tmpl.Execute(fdFile, tplData); err != nil {
			return err
		}

		err = (*workstation).Transfer(ctx, fmt.Sprintf("/tmp/%s", configFile), "/opt/tidb/", false, 0)
		if err != nil {
			return err
		}
	}

	// 5. Render the ddl templates to tidb/aurora/sql server
	sqlFiles := []string{"ontime_ms.ddl", "ontime_mysql.ddl", "ontime_tidb.ddl"}
	for _, sqlFile := range sqlFiles {
		err = (*workstation).Transfer(ctx, fmt.Sprintf("embed/templates/sql/%s", sqlFile), "/opt/tidb/sql/", false, 0)
		if err != nil {
			return err
		}
	}

	// 6. Send the access key to workstation
	err = (*workstation).Transfer(ctx, c.awsWSConfigs.KeyFile, "~/.ssh/id_rsa", false, 0)
	if err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false)
	if err != nil {
		return err
	}

	// Format the disk and run
	for _, _ip := range tplData.TiKV {
		_remoteNode, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: _ip.IPAddress, User: (*c.awsWSConfigs).UserName, KeyFile: (*c.awsWSConfigs).KeyFile, Proxy: &executor.SSHConfig{Host: wsInfo.PublicIpAddress, User: (*c.awsWSConfigs).UserName, Port: 22, KeyFile: (*c.awsWSConfigs).KeyFile}}, []string{})

		if err != nil {
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "mkdir -p ~/tidb", false)
		if err != nil {
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "mkdir -p /opt/scripts", true)
		if err != nil {
			return err
		}

		if err = _remoteNode.TransferTemplate(ctx, "templates/scripts/fdisk.sh.tpl", "/opt/scripts/fdisk.sh", "0755", nil, true, 20); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "/opt/scripts/fdisk.sh", true)
		if err != nil {
			return err
		}
	}

	for _, _ip := range tplData.TiFlash {
		_remoteNode, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: _ip, User: (*c.awsWSConfigs).UserName, KeyFile: (*c.awsWSConfigs).KeyFile, Proxy: &executor.SSHConfig{Host: wsInfo.PublicIpAddress, User: (*c.awsWSConfigs).UserName, Port: 22, KeyFile: (*c.awsWSConfigs).KeyFile}}, []string{})

		if err != nil {
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "mkdir -p ~/tidb", false)
		if err != nil {
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "mkdir -p /opt/scripts", true)
		if err != nil {
			return err
		}

		if err = _remoteNode.TransferTemplate(ctx, "templates/scripts/fdisk.sh.tpl", "/opt/scripts/fdisk.sh", "0755", nil, true, 20); err != nil {
			fmt.Printf("Error: %#v \n\n\n", err)
			return err
		}

		_, _, err = _remoteNode.Execute(ctx, "/opt/scripts/fdisk.sh", true)
		if err != nil {
			return err
		}
	}

	// 7. Add limit configuration, otherwise the configuration will impact the performance test with heavy load.
	/*
	 * hard nofile 65535
	 * soft nofile 65535
	 */
	err = (*workstation).Transfer(ctx, "embed/templates/config/limits.conf", "/tmp", false, 0)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, `mv /tmp/limits.conf /etc/security/limits.conf`, true)
	if err != nil {
		return err

	}

	for idx := 0; idx < 10; idx++ {
		stdout, _, err = (*workstation).Execute(ctx, `lslocks --json`, true)
		if err != nil {
			return err
		}
		aptLocked, err := LookupAptLock("apt-get", stdout)
		if err != nil {
			return err
		}

		if aptLocked == true {
			time.Sleep(30 * time.Second)
			continue
		}
		stdout, _, err = (*workstation).Execute(ctx, `apt-get update`, true)
		if err != nil {
			return err
		}

		if err := installPKGs(workstation, ctx, []string{"mariadb-client-10.3"}); err != nil {
			return err
		}

		break
	}

	if _, _, err = (*workstation).Execute(ctx, `curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh`, false); err != nil {
		return err
	}

	dbInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "sqlserver")
	if err != nil {
		if err.Error() == "No RDB Instance found(No matched name)" {
			return nil
		}
		return err
	}

	deployFreetds(*workstation, ctx, "REPLICA", dbInstance.Endpoint.Address, dbInstance.Endpoint.Port)

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`printf \"IF (db_id('cdc_test') is null)\n  create database cdc_test;\ngo\n\" | tsql -S REPLICA -p %d -U %s -P %s`, dbInstance.Endpoint.Port, dbInstance.MasterUsername, "1234Abcd"), true)
	if err != nil {
		return err
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
