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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

type ScaleTiDB struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	awsTopoConfig  *spec.AwsTopoConfigs
	subClusterType string
}

// Execute implements the Task interface
func (c *ScaleTiDB) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	tidbClusterInfo, err := getTiDBClusterInfo(workstation, ctx, clusterName)
	if err != nil {
		fmt.Printf("The error for fetching cluster info is <%s> \n\n\n", err.Error())
	}

	// 2. Get all the nodes from tag definition
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	pdMap := make(map[string]bool)
	tikvMap := make(map[string]bool)
	tidbMap := make(map[string]bool)
	ticdcMap := make(map[string]bool)
	dmMap := make(map[string]bool)
	//	fmt.Printf("The original instances info is <%#v> \n\n\n\n\n\n", c.oldInstances.Reservations)

	for _, instance := range tidbClusterInfo.Instances {
		if instance.Role == "pd" {
			pdMap[instance.Host] = true
		}
		if instance.Role == "tikv" {
			tikvMap[instance.Host] = true
		}
		if instance.Role == "tidb" {
			tidbMap[instance.Host] = true
		}
		if instance.Role == "cdc" {
			ticdcMap[instance.Host] = true
		}
		//fmt.Printf("The instance is <%#v> \n\n\n", instance)
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	fmt.Printf("All the pd nodes are <%#v> \n\n\n\n\n\n", pdMap)
	var tplData TplTiupData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "pd" && !pdMap[instance.PrivateIpAddress] {
					tplData.PD = append(tplData.PD, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "tidb" && !tidbMap[instance.PrivateIpAddress] {
					tplData.TiDB = append(tplData.TiDB, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "tikv" && !tikvMap[instance.PrivateIpAddress] {
					tplData.TiKV = append(tplData.TiKV, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" && !ticdcMap[instance.PrivateIpAddress] {
					tplData.TiCDC = append(tplData.TiCDC, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "dm" && !dmMap[instance.PrivateIpAddress] {
					tplData.DM = append(tplData.DM, instance.PrivateIpAddress)
				}
			}
		}
	}
	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplData.String()))

	buffer := &bytes.Buffer{}
	buffer.WriteString("---\n")
	if len(tplData.PD) > 0 {
		buffer.WriteString("pd_servers:\n")
		for _, ip := range tplData.PD {
			buffer.WriteString("  - host: ")
			buffer.WriteString(ip)
			buffer.WriteString("\n")
		}
	}
	if len(tplData.TiDB) > 0 {
		buffer.WriteString("tidb_servers:\n")
		for _, ip := range tplData.TiDB {
			buffer.WriteString("  - host: ")
			buffer.WriteString(ip)
			buffer.WriteString("\n")
		}
	}
	if len(tplData.TiKV) > 0 {
		buffer.WriteString("tikv_servers:\n")
		for _, ip := range tplData.TiKV {
			buffer.WriteString("  - host: ")
			buffer.WriteString(ip)
			buffer.WriteString("\n")
		}
	}
	if len(tplData.TiCDC) > 0 {
		buffer.WriteString("cdc_servers:\n")
		for _, ip := range tplData.TiCDC {
			buffer.WriteString("  - host: ")
			buffer.WriteString(ip)
			buffer.WriteString("\n")
		}
	}
	// if len(tplData.DM) > 0 {
	// 	buffer.WriteString("dm_servers:\n")
	// 	for _, ip := range tplData.DM {
	// 		buffer.WriteString("  - host: ")
	// 		buffer.WriteString(ip)
	// 		buffer.WriteString("\n")
	// 	}
	// }
	if err := os.WriteFile("/tmp/scale.yaml", buffer.Bytes(), os.FileMode(0644)); err != nil {
		return err
	}
	err = (*workstation).Transfer(ctx, "/tmp/scale.yaml", "/opt/tidb/scale.yaml", false, 0)
	if err != nil {
		return err
	}

	// _, _, err = (*workstation).Execute(ctx, "chmod 644 /opt/tidb/scale.yaml", false, 0)
	// if err != nil {
	// 	return err
	// }

	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster scale-out %s  -y %s`, clusterName, "/opt/tidb/scale.yaml"), false, 300*time.Second)
	if err != nil {
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *ScaleTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ScaleTiDB) String() string {
	return fmt.Sprintf("Echo: Scaling TiDB")
}
