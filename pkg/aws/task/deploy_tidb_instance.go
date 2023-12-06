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

	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"go.uber.org/zap"

	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	elbutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/elb"
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

	ec2api *ec2utils.EC2API
}

// Execute implements the Task interface
func (c *DeployTiDBInstance) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 02. Extract TiDB Cluster EC2 instances
	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType
	mapArgs["subClusterType"] = c.subClusterType

	var err error
	c.ec2api, err = ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}

	instances, err := c.ec2api.ExtractEC2Instances()
	if err != nil {
		return err
	}

	// TODO: remove wsExe
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

	// If there is no VM EC2 nodes, skip the vm instal
	if (len((*instances)["VM"])) > 0 {
		if err := c.installVM(ctx, (*instances)["VM"]); err != nil {
			return err
		}
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

func (c *DeployTiDBInstance) installVM(ctx context.Context, nodes []interface{}) error {
	for _, nodeIP := range nodes {
		// 01. Install binary to each node
		if err := c.workstation.RunSerialCmdsOnRemoteNode(nodeIP.(string), []string{
			"wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.95.1/victoria-metrics-linux-amd64-v1.95.1-cluster.tar.gz -O - | sudo tar -xz -C /usr/local/bin",
			"wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.95.1/vmutils-linux-amd64-v1.95.1.tar.gz -O - | sudo tar -xz -C /usr/local/bin"}, false); err != nil {
			return err
		}

	}

	_params := make(map[interface{}]interface{})

	var arrVMInsert []string
	var arrVMSelect []string
	for _, nodeIP := range nodes {
		arrVMInsert = append(arrVMInsert, fmt.Sprintf("%s:8400", nodeIP.(string)))
		arrVMSelect = append(arrVMSelect, fmt.Sprintf("%s:8401", nodeIP.(string)))
	}
	_params["VMINSERT"] = strings.Join(arrVMInsert, ",")
	_params["VMSELECT"] = strings.Join(arrVMSelect, ",")

	for _, nodeIP := range nodes {
		// Copy the prometheus config file for mvagent deployment
		if err := c.workstation.TransferWSFile2Remote(nodeIP.(string), "/home/admin/tidb/tidb-deploy/prometheus-9090/conf/prometheus.yml", "/etc/prometheus.yml", true); err != nil {
			return err
		}

		// 02. Prepare system file to each node
		_params["STORAGE_IP"] = nodeIP

		if err := c.workstation.RenderTemplate2Remote(nodeIP.(string), "templates/systemd/vmstorage.service.tpl", "/etc/systemd/system/vmstorage.service", _params, true); err != nil {
			return err
		}

		if err := c.workstation.RenderTemplate2Remote(nodeIP.(string), "templates/systemd/vmselect.service.tpl", "/etc/systemd/system/vmselect.service", _params, true); err != nil {
			return err
		}

		if err := c.workstation.RenderTemplate2Remote(nodeIP.(string), "templates/systemd/vminsert.service.tpl", "/etc/systemd/system/vminsert.service", _params, true); err != nil {
			return err
		}

		if err := c.workstation.RenderTemplate2Remote(nodeIP.(string), "templates/systemd/vmagent.service.tpl", "/etc/systemd/system/vmagent.service", _params, true); err != nil {
			return err
		}
	}

	// 03. Start service.
	for _, nodeIP := range nodes {
		if err := c.workstation.RunSerialCmdsOnRemoteNode(nodeIP.(string), []string{
			"systemctl start vmstorage.service",
			"systemctl start vminsert.service",
			"systemctl start vmselect.service",
		}, true); err != nil {
			return err
		}

	}

	// 04. Add NLB to three nodes
	dnsName, err := c.createVMEndpoint(ctx, nodes)
	if err != nil {
		return err
	}

	_params["VMEndpoint"] = dnsName
	for _, nodeIP := range nodes {
		if err := c.workstation.RenderTemplate2Remote(nodeIP.(string), "templates/systemd/vmagent.service.tpl", "/etc/systemd/system/vmagent.service", _params, true); err != nil {
			return err
		}

		if err := c.workstation.RunSerialCmdsOnRemoteNode(nodeIP.(string), []string{
			"sed -i -E '/evaluation_interval/d' /etc/prometheus.yml",
			"sed -i -E '/rule_files/d' /etc/prometheus.yml",
			"sed -i -E '/.yml/d' /etc/prometheus.yml",
			"systemctl daemon-reload",
			"systemctl start vmagent.service",
		}, true); err != nil {
			return err
		}
	}

	return nil
}

// Create taget group for VM and NLB to VMInsert and VMSelect
// 01. Create target group for vminsert
// 02. Create target group for vmselect
// 03. Create NLB
// 04. Attach vminsert target group to NLB
// 05. Attach vmselect target group to NLB
func (c *DeployTiDBInstance) createVMEndpoint(ctx context.Context, nodes []interface{}) (*string, error) {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType
	mapArgs["subClusterType"] = c.subClusterType

	elbapi, err := elbutils.NewELBAPI(&mapArgs)
	if err != nil {
		return nil, err
	}

	vpcInfo, err := c.ec2api.GetVpc()
	if err != nil {
		return nil, err
	}

	vmNodesID, err := c.ec2api.ExtractEC2InstancesIDByComp("vm")
	if err != nil {
		return nil, err
	}

	subnets, err := c.ec2api.GetSubnets("private")
	if err != nil {
		return nil, err
	}

	sg, err := c.ec2api.GetSG("private")
	if err != nil {
		return nil, err
	}

	// Input: vpc-id/port/type(tcp/http)/nodes
	if _, err := elbapi.CreateTargetGroup(clusterName, "vminsert", vpcInfo.VpcId, 8480, elbtypes.ProtocolEnumHttp, subnets, vmNodesID, sg); err != nil {
		return nil, err
	}

	if _, err = elbapi.CreateTargetGroup(clusterName, "vmselect", vpcInfo.VpcId, 8481, elbtypes.ProtocolEnumHttp, subnets, vmNodesID, sg); err != nil {
		return nil, err
	}

	// 05. deploy the vmagent
	_, dnsName, _, err := elbapi.GetNLBInfo(clusterName, "vmendpoint")
	if err != nil {
		return nil, err
	}
	if dnsName == nil {
		return nil, errors.New(fmt.Sprintf("Failed to fetch the DNSName of %s-%s", clusterName, "vmendpoint"))
	}

	if err := c.workstation.RunSerialCmds([]string{
		fmt.Sprintf("sed -i -E 's|url: http://(.*):(9090)|url: http://%s:8481/select/0/prometheus|g' /home/admin/tidb/tidb-deploy/grafana-3000/provisioning/datasources/datasource.yml", *dnsName),
		"systemctl restart grafana-3000",
	}, true); err != nil {
		return nil, err
	}

	return dnsName, nil
}
