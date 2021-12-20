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
	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
	"os"
	"path"
	"strings"
	"text/template"
)

type DeployTiDB struct {
	user           string
	host           string
	awsWSConfigs   *spec.AwsWSConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

type TplTiupData struct {
	PD      []string
	TiDB    []string
	TiKV    []string
	TiCDC   []string
	DM      []string
	Monitor []string
}

func (t TplTiupData) String() string {
	return fmt.Sprintf("PD: %s  |  TiDB: %s  |  TiKV: %s  |  TiCDC: %s  |  DM: %s  |  Monitor:%s", strings.Join(t.PD, ","), strings.Join(t.TiDB, ","), strings.Join(t.TiKV, ","), strings.Join(t.TiCDC, ","), strings.Join(t.DM, ","), strings.Join(t.Monitor, ","))
}

// Execute implements the Task interface
func (c *DeployTiDB) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	// 1. Get all the workstation nodes
	workstation, err := getWSExecutor(local, ctx, c.clusterName, c.clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 2. Get all the nodes from tag definition
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := local.Execute(ctx, command, false)
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
				if tag["Key"] == "Component" && tag["Value"] == "tikv" {
					tplData.TiKV = append(tplData.TiKV, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" {
					tplData.TiCDC = append(tplData.TiCDC, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "dm" {
					tplData.DM = append(tplData.DM, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplData.Monitor = append(tplData.Monitor, instance.PrivateIpAddress)

				}
			}
		}
	}
	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplData.String()))

	// 3. Make all the necessary folders
	if _, _, err := (*workstation).Execute(ctx, `mkdir -p /opt/tidb/sql`, true); err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, `chown -R admin:admin /opt/tidb`, true); err != nil {
		return err
	}

	// 4. Deploy all tidb templates
	configFiles := []string{"cdc-task.toml", "dm-cluster.yml", "dm-source.yml", "dm-task.yml", "dm-task.yml", "tidb-cluster.yml"}
	for _, configFile := range configFiles {
		fmt.Printf("The config file to copy is <%s> \n\n\n", configFile)

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
			fmt.Printf("The error is <%#v> \n\n\n", err)
			return err
		}
	}

	// 5. Render the ddl templates to tidb/aurora/sql server
	sqlFiles := []string{"ontime_ms.ddl", "ontime_mysql.ddl", "ontime_tidb.ddl"}
	for _, sqlFile := range sqlFiles {
		err = (*workstation).Transfer(ctx, fmt.Sprintf("embed/templates/sql/%s", sqlFile), "/opt/tidb/sql/", false, 0)
		if err != nil {
			fmt.Printf("The error is <%#v> \n\n\n", err)
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
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
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
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh`, false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `apt-get install -y mariadb-client-10.3`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	// MS Sqlserver setup
	node, err := getEC2Nodes(local, ctx, c.clusterName, c.clusterType, "sqlserver")
	if err != nil {
		return err
	}
	if len(*node) == 0 {
		return nil
	}

	deployMS2008(*workstation, ctx, (*node)[0].PrivateIpAddress)

	stdout, _, err = (*workstation).Execute(ctx, `printf \"IF (db_id('cdc_test') is null)\n  create database cdc_test;\ngo\n\" | tsql -S REPLICA -p 1433 -U sa -P 1234@Abcd`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
