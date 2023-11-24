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

	// "errors"
	"fmt"
	"regexp"
	"time"

	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"go.uber.org/zap"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

func (b *Builder) DeployTiDBInstance(awsWSConfigs *spec.AwsWSConfigs, subClusterType, tidbVersion string, enableAuditLog bool, workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DeployTiDBInstance{
		subClusterType: subClusterType,
		awsWSConfigs:   awsWSConfigs,
		tidbVersion:    tidbVersion,
		enableAuditLog: enableAuditLog,
		workstation:    workstation,
	})
	return b
}

type TiDBClusterInfo struct {
	Name       string `json:"name"`
	User       string `json:"user"`
	Version    string `json:"version"`
	Path       string `json:"path"`
	PrivateKey string `json:"private_key"`
}

type TiDBClusterInfos struct {
	TiDBClusterInfos []TiDBClusterInfo `json:"clusters"`
}

type TiDBClusterMeta struct {
	ClusterType    string `json:"cluster_type"`
	ClusterName    string `json:"cluster_name"`
	ClusterVersion string `json:"cluster_version"`
	DeployUser     string `json:"deploy_user"`
	SshType        string `json:"ssh_type"`
	TlsEnabled     bool   `json:"tls_enabled"`
	DashboardUrl   string `json:"dashboard_url"`
}

type TiDBClusterComponent struct {
	Id            string `json:"id"`
	Role          string `json:"role"`
	Host          string `json:"host"`
	Ports         string `json:"ports"`
	OsArch        string `json:"os_arch"`
	Status        string `json:"status"`
	Since         string `json:"since"`
	DataDir       string `json:"data_dir"`
	DeployDir     string `json:"deploy_dir"`
	ComponentName string `json:"ComponentName"`
	Port          int    `json:"Port"`
}

type TiDBClusterDetail struct {
	TiDBClusterMeta TiDBClusterMeta        `json:"cluster_meta"`
	Instances       []TiDBClusterComponent `json:"instances"`
}

type DeployTiDBInstance struct {
	awsWSConfigs *spec.AwsWSConfigs
	workstation  *ws.Workstation

	subClusterType string
	tidbVersion    string
	enableAuditLog bool
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *DeployTiDBInstance) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	wsExe, err := c.workstation.GetExecutor()
	if err != nil {
		return err
	}

	tiupClusterCmd := fmt.Sprintf("/home/%s/.tiup/bin/tiup cluster", c.awsWSConfigs.UserName)

	stdout, _, err := (*wsExe).Execute(ctx, fmt.Sprintf(`%s list --format json `, tiupClusterCmd), false)
	if err != nil {
		return err
	}

	var tidbClusterInfos TiDBClusterInfos
	if err = json.Unmarshal(stdout, &tidbClusterInfos); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return err
	}

	clusterExists := false
	for _, tidbClusterInfo := range tidbClusterInfos.TiDBClusterInfos {
		if tidbClusterInfo.Name == clusterName {
			clusterExists = true
			break
		}
	}

	if clusterExists == false {

		stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s deploy %s %s /opt/tidb/tidb-cluster.yml -y`, tiupClusterCmd, clusterName, c.tidbVersion), false, 300*time.Second)
		if err != nil {
			return err
		}

		if c.enableAuditLog == true && c.tidbVersion < "v7.1.0" {
			binPlugin := fmt.Sprintf("enterprise-plugin-%s-linux-amd64", c.tidbVersion)

			stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s exec %s --command "mkdir -p {{.DeployDir}}/plugin"`, tiupClusterCmd, clusterName), false, 300*time.Second)
			if err != nil {
				return err
			}

			stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s push %s /tmp/%s/bin/audit-1.so {{.DeployDir}}/plugin/audit-1.so`, tiupClusterCmd, clusterName, binPlugin), false, 300*time.Second)
			if err != nil {
				return err
			}

			stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s push %s /tmp/%s/bin/whitelist-1.so {{.DeployDir}}/plugin/whitelist-1.so`, tiupClusterCmd, clusterName, binPlugin), false, 300*time.Second)
			if err != nil {
				return err
			}
		}

		stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s start %s`, tiupClusterCmd, clusterName), false, 300*time.Second)
		if err != nil {
			return err
		}

		if err = awsutils.WaitResourceUntilExpectState(60*time.Second, 60*time.Minute, func() (bool, error) {
			stdout, _, err := (*wsExe).Execute(ctx, fmt.Sprintf(`%s display %s --format json `, tiupClusterCmd, clusterName), false)
			if err != nil {
				return false, err
			}

			var tidbClusterDetail TiDBClusterDetail
			if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
				zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
				return false, err
			}

			for _, component := range tidbClusterDetail.Instances {
				matched, err := regexp.MatchString(`^Up.*`, component.Status)
				if err != nil {
					return false, err
				}

				if matched == false {
					return false, nil
				}
			}
			return true, nil
		}); err != nil {
			return err
		}

	} else {
		stdout, _, err := (*wsExe).Execute(ctx, fmt.Sprintf(`%s display %s --format json `, tiupClusterCmd, clusterName), false)
		if err != nil {
			return err
		}

		var tidbClusterDetail TiDBClusterDetail
		if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
			zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
			return nil
		}
		for _, component := range tidbClusterDetail.Instances {
			if component.Status != "Up" {
				stdout, _, err = (*wsExe).Execute(ctx, fmt.Sprintf(`%s start %s --node %s `, tiupClusterCmd, clusterName, component.Id), false)
				if err != nil {
					return err
				}
			}
		}

	}

	if err := c.workstation.InstallMySQLShell(); err != nil {
		return err
	}

	if err := c.workstation.DeployTiDBInfo(clusterName); err != nil {
		return err
	}

	// The audit log feature is merged into the TiDB bin after v7.1.0.
	// If the version is before v7.1.0, to use the audit log, the plugins set must be set.
	if c.enableAuditLog == true && c.tidbVersion < "v7.1.0" {
		res, err := c.workstation.QueryTiDB("mysql", "select count(*) cnt from mysql.tidb_audit_table_access where user = '.*' and db = '.*' and tbl = '.*'")
		if err != nil {
			return err
		}

		if int((*res)[0]["cnt"].(float64)) == 0 {
			if err := c.workstation.ExecuteTiDB("mysql", "insert into mysql.tidb_audit_table_access (user, db, tbl, access_type) values ('.*', '.*', '.*', '')"); err != nil {
				return err
			}

			if err := c.workstation.ExecuteTiDB("mysql", "admin plugins enable whitelist"); err != nil {
				return err
			}

			if err := c.workstation.ExecuteTiDB("mysql", "admin plugins enable audit"); err != nil {
				return err
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployTiDBInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiDBInstance) String() string {
	return fmt.Sprintf("Echo: Deploying TiDB instance ")
}
