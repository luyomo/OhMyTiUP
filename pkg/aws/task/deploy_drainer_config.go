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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"text/template"
	//"time"
	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type DeployDrainConfig struct {
	pexecutor        *ctxt.Executor
	awsWSConfigs     *spec.AwsWSConfigs
	awsOracleConfigs *spec.AwsOracleConfigs
	drainerReplicate *spec.DrainerReplicate
}

// Execute implements the Task interface
func (c *DeployDrainConfig) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Extract Drainer IP by TiDB Cluster info
	nodes, err := utils.ExtractTiDBClusterNodes(clusterName, clusterType, "tidb")
	if err != nil {
		return err
	}

	// 2. Extract Oracle info from RDS
	oracleInstanceInfos, err := utils.ExtractInstanceRDSInfo(clusterName, clusterType, "oracle")
	if err != nil {
		return err
	}

	// 3. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 4. install the required package
	if err := installPKGs(workstation, ctx, []string{"zip"}); err != nil {
		return err
	}

	// 5. Generate the drainer config file
	type TPLParams struct {
		DrainerNodeIP string
		PDUrl         string
		DBName        string
		DBHost        string
		DBUser        string
		DBPassword    string
		DBPort        int64
		DBInstance    string
		ReplicateDB   string
	}

	var tplParams TPLParams
	tplParams.DrainerNodeIP = strings.Join((*nodes).Drainer, " ")
	var pdUrl []string
	for _, pd := range (*nodes).PD {
		pdUrl = append(pdUrl, fmt.Sprintf("http://%s:2379", pd))
	}
	if len(*oracleInstanceInfos) == 0 {
		return errors.New("No oracle instance")
	}
	tplParams.PDUrl = strings.Join(pdUrl, ",")
	tplParams.ReplicateDB = c.drainerReplicate.ReplicateDB
	tplParams.DBName = (*oracleInstanceInfos)[0].DBName
	tplParams.DBHost = (*oracleInstanceInfos)[0].EndPointAddress
	tplParams.DBPort = (*oracleInstanceInfos)[0].DBPort
	tplParams.DBUser = (*oracleInstanceInfos)[0].DBUserName
	tplParams.DBPassword = c.awsOracleConfigs.DBPassword
	tplParams.DBInstance = c.awsOracleConfigs.DBInstanceName

	if _, _, err := (*workstation).Execute(ctx, `install -d -m 0755 -o admin -g admin /opt/scripts/etc`, true); err != nil {
		return err
	}

	fdFile, err := os.Create("/tmp/drainer_oracle.toml")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	err = os.Chmod("/tmp/drainer_oracle.toml", 0644)
	if err != nil {
		return err
	}

	fp := path.Join("templates", "config", "drainer.oracle.toml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(fdFile, tplParams); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/drainer_oracle.toml", "/opt/scripts/etc/drainer_oracle.toml", false, 0)
	if err != nil {
		return err
	}

	for _, drainerIP := range nodes.Drainer {
		// 6. Copy the drainer config file to drainer server
		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "sudo mkdir -p /etc/drainer"`, drainerIP), false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /opt/scripts/etc/drainer_oracle.toml %s:/tmp/drainer_oracle.toml`, drainerIP), false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "sudo mv /tmp/drainer_oracle.toml /etc/drainer/drainer_oracle.toml"`, drainerIP), false); err != nil {
			return err
		}

		// 7. Install TiDB bin in the drainer node
		if _, _, err := (*workstation).Execute(ctx, "/opt/scripts/install_tidb.sh", false); err != nil {
			return err
		}

		// 8. Install Oracle Client in the drainer node
		if _, _, err := (*workstation).Execute(ctx, "/opt/scripts/install_oracle_client.sh", false); err != nil {
			return err
		}

		// 9. Copy the drainer_oracle_resource.sql
		fdFile, err := os.Create("/tmp/install_oracle_resource.sh")
		if err != nil {
			return err
		}
		defer fdFile.Close()

		err = os.Chmod("/tmp/install_oracle_resource.sh", 0755)
		if err != nil {
			return err
		}

		dbInitFile := path.Join("templates", "scripts", "install_oracle_resource.sh.tpl")

		tpl, err := embed.ReadTemplate(dbInitFile)
		if err != nil {
			return err
		}

		tmpl, err := template.New("test").Parse(string(tpl))
		if err != nil {
			return err
		}

		if err := tmpl.Execute(fdFile, tplParams); err != nil {
			return err
		}

		err = (*workstation).Transfer(ctx, "/tmp/install_oracle_resource.sh", "/opt/scripts/install_oracle_resource.sh", false, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /opt/scripts/install_oracle_resource.sh %s:/tmp/install_oracle_resource.sh`, drainerIP), false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, "/opt/scripts/install_tidb.sh", false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s /tmp/install_oracle_resource.sh`, drainerIP), false); err != nil {
			return err
		}

		// 10. Copy the drainer.service
		fdFile, err = os.Create("/tmp/drainer.service")
		if err != nil {
			return err
		}
		defer fdFile.Close()

		err = os.Chmod("/tmp/drainer.service", 0755)
		if err != nil {
			return err
		}

		dbInitFile = path.Join("templates", "config", "drainer.service")

		tpl, err = embed.ReadTemplate(dbInitFile)
		if err != nil {
			return err
		}

		tmpl, err = template.New("test").Parse(string(tpl))
		if err != nil {
			return err
		}

		if err := tmpl.Execute(fdFile, tplParams); err != nil {
			return err
		}

		err = (*workstation).Transfer(ctx, "/tmp/drainer.service", "/opt/scripts/drainer.service", false, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /opt/scripts/drainer.service %s:/tmp/drainer.service`, drainerIP), false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "sudo mv /tmp/drainer.service /etc/systemd/system/drainer.service"`, drainerIP), false); err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "sudo systemctl start drainer.service"`, drainerIP), false); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployDrainConfig) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployDrainConfig) String() string {
	return fmt.Sprintf("Echo: Deploying Drainer config ... ... ")
}
