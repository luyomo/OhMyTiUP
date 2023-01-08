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
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"go.uber.org/zap"
	"time"
)

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
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	tidbVersion    string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *DeployTiDBInstance) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=16\"", clusterName, clusterType)
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

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			cntInstance++
			theInstance = instance
		}
	}

	command = fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: theInstance.PublicIpAddress, User: c.awsWSConfigs.UserName, KeyFile: c.clusterInfo.keyFile}, []string{})
	if err != nil {
		return err
	}

	stdout, _, err = wsexecutor.Execute(ctx, fmt.Sprintf(`/home/%s/.tiup/bin/tiup cluster list --format json `, c.awsWSConfigs.UserName), false)
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

		stdout, _, err = wsexecutor.Execute(ctx, fmt.Sprintf(`/home/%s/.tiup/bin/tiup cluster deploy %s %s /opt/tidb/tidb-cluster.yml -y`, c.awsWSConfigs.UserName, clusterName, c.tidbVersion), false, 300*time.Second)
		if err != nil {
			return err
		}

		stdout, _, err = wsexecutor.Execute(ctx, fmt.Sprintf(`/home/%s/.tiup/bin/tiup cluster start %s`, c.awsWSConfigs.UserName, clusterName), false, 300*time.Second)
		if err != nil {
			return err
		}
	} else {
		stdout, _, err := wsexecutor.Execute(ctx, fmt.Sprintf(`/home/%s/.tiup/bin/tiup cluster display %s --format json `, c.awsWSConfigs.UserName, clusterName), false)
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
				stdout, _, err = wsexecutor.Execute(ctx, fmt.Sprintf(`/home/%s/.tiup/bin/tiup cluster start %s --node %s `, c.awsWSConfigs.UserName, clusterName, component.Id), false)
				if err != nil {
					return err
				}
			}
		}

	}

	nlb, err := getNLB(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
	if err != nil {
		return err
	}

	var dbInfo DBInfo
	dbInfo.DBHost = *(*nlb).DNSName
	dbInfo.DBPort = 4000
	dbInfo.DBUser = "root"

	_, _, err = wsexecutor.Execute(ctx, "mkdir -p /opt/scripts", true)
	if err != nil {
		return err
	}

	err = wsexecutor.TransferTemplate(ctx, "templates/config/db-info.yml.tpl", "/opt/tidb-db-info.yml", "0644", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = wsexecutor.TransferTemplate(ctx, "templates/scripts/run_mysql_query.sh.tpl", "/opt/scripts/run_tidb_query", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = wsexecutor.TransferTemplate(ctx, "templates/scripts/run_mysql_from_file.sh.tpl", "/opt/scripts/run_tidb_from_file", "0755", dbInfo, true, 0)
	if err != nil {
		return err
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
