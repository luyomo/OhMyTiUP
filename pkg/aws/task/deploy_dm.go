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
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

type DeployDM struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

type TplTiupDMData struct {
	DMMaster     []string
	DMWorker     []string
	Monitor      []string
	Grafana      []string
	AlertManager []string
}

func (t TplTiupDMData) String() string {
	return fmt.Sprintf("DM Master: %s  |  DMWorker:%s  | Monitor:%s | AlertManager: %s | Grafana: %s", strings.Join(t.DMMaster, ","), strings.Join(t.DMWorker, ","), strings.Join(t.Monitor, ","), strings.Join(t.Grafana, ","), strings.Join(t.AlertManager, ","))
}

// Execute implements the Task interface
func (c *DeployDM) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

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

	var tplDMData TplTiupDMData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {

				if tag["Key"] == "Component" && tag["Value"] == "dm-master" {
					tplDMData.DMMaster = append(tplDMData.DMMaster, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "dm-worker" {
					tplDMData.DMWorker = append(tplDMData.DMWorker, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.Grafana = append(tplDMData.Grafana, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.Monitor = append(tplDMData.Monitor, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.AlertManager = append(tplDMData.AlertManager, instance.PrivateIpAddress)

				}
			}
		}
	}
	zap.L().Debug("AWS WS Config:", zap.String("Monitoring", c.awsWSConfigs.EnableMonitoring))
	if c.awsWSConfigs.EnableMonitoring == "enabled" {
		workstation, err := getWorkstation(*c.pexecutor, ctx, clusterName, clusterType)
		if err != nil {
			return err
		}
		tplDMData.Monitor = append(tplDMData.Monitor, workstation.PrivateIpAddress)
	}
	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplDMData.String()))

	// 3. Make all the necessary folders
	if _, _, err := (*workstation).Execute(ctx, `mkdir -p /opt/tidb/sql`, true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`chown -R %s:%s /opt/tidb`, c.awsWSConfigs.UserName, c.awsWSConfigs.UserName), true); err != nil {
		return err
	}

	// 4. Deploy all tidb templates
	configFiles := []string{"dm-task.yml", "dm-cluster.yml"}
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

		if err := tmpl.Execute(fdFile, tplDMData); err != nil {
			return err
		}

		err = (*workstation).Transfer(ctx, fmt.Sprintf("/tmp/%s", configFile), "/opt/tidb/", false, 0)
		if err != nil {
			return err
		}
	}

	// 6. Send the access key to workstation
	err = (*workstation).Transfer(ctx, c.clusterInfo.keyFile, "~/.ssh/id_rsa", false, 0)
	if err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false)
	if err != nil {
		return err
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

	stdout, _, err = (*workstation).Execute(ctx, `apt-get update`, true)
	if err != nil {
		return err
	}
	// stdout, _, err = (*workstation).Execute(ctx, `yum update`, true)
	// if err != nil {
	// 	return err
	// }

	stdout, _, err = (*workstation).Execute(ctx, `curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh`, false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	if err := installPKGs(workstation, ctx, []string{"mariadb-client-10.3"}); err != nil {
		return err
	}

	dmClusterInfo, err := getDMClusterInfo(workstation, ctx, clusterName)
	if err != nil {
		return err
	}

	if dmClusterInfo == nil {
		stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm deploy %s %s %s -y", "aurora2tidbcloudtest01", "v6.1.0", "/opt/tidb/dm-cluster.yml"), false)
		if err != nil {
			fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
			return err
		}
	}

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm start %s", "aurora2tidbcloudtest01"), false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	if err = (*workstation).Transfer(ctx, "/opt/db-info.yml", "/tmp/db-info.yml", true, 1024); err != nil {
		return err
	}

	type SourceData struct {
		MySQLHost     string `yaml:"Host"`
		MySQLPort     int    `yaml:"Port"`
		MySQLUser     string `yaml:"User"`
		MySQLPassword string `yaml:"Password"`

		SourceName string
	}

	sourceData := SourceData{}

	yfile, err := ioutil.ReadFile("/tmp/db-info.yml")
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(yfile, &sourceData); err != nil {
		return err
	}
	sourceData.SourceName = clusterName
	fmt.Printf("The parameteers are <%#v> \n\n\n\n", sourceData)
	err = (*workstation).TransferTemplate(ctx, "templates/config/dm-source.yml.tpl", "/tmp/dm-source.yml", "0644", sourceData, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, "mv /tmp/dm-source.yml /opt/tidb/", true); err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s:8261 operate-source create /opt/tidb/dm-source.yml", tplDMData.DMMaster[0]), false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	// stdout, _, err = (*workstation).Execute(ctx, `apt-get install -y mariadb-client-10.3`, true)
	// if err != nil {
	// 	return err
	// }

	// stdout, _, err = (*workstation).Execute(ctx, `yum install -y mariadb.x86_64`, true)
	// if err != nil {
	// 	return err
	// }

	// dbInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "sqlserver")
	// if err != nil {
	// 	if err.Error() == "No RDB Instance found(No matched name)" {
	// 		return nil
	// 	}
	// 	fmt.Printf("The error is <%#v> \n\n\n", dbInstance)
	// 	return err
	// }

	// deployFreetds(*workstation, ctx, "REPLICA", dbInstance.Endpoint.Address, dbInstance.Endpoint.Port)

	// stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`printf \"IF (db_id('cdc_test') is null)\n  create database cdc_test;\ngo\n\" | tsql -S REPLICA -p %d -U %s -P %s`, dbInstance.Endpoint.Port, dbInstance.MasterUsername, "1234Abcd"), true)
	// if err != nil {
	// 	return err
	// }

	return nil
}

// Rollback implements the Task interface
func (c *DeployDM) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployDM) String() string {
	return fmt.Sprintf("Echo: Deploying DM")
}
