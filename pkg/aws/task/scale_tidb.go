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

	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
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
		return err

	}

	// 2. Get all the nodes from tag definition
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false, 600*time.Second)
	if err != nil {
		return err
	}

	pdMap := make(map[string]bool)
	tikvMap := make(map[string]bool)
	tidbMap := make(map[string]bool)
	ticdcMap := make(map[string]bool)
	dmMap := make(map[string]bool)

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
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	activeNodesMap := make(map[string]map[string][][]string)
	var tplData TplTiupData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "pd" {
					if !pdMap[instance.PrivateIpAddress] {
						tplData.PD = append(tplData.PD, instance.PrivateIpAddress)
					} else {
						if activeNodesMap["pd"] == nil {
							activeNodesMap["pd"] = make(map[string][][]string)
						}
						nodeId := SearchTiDBNode(tidbClusterInfo.Instances, instance.PrivateIpAddress)
						activeNodesMap["pd"][instance.SubnetId] = append(activeNodesMap["pd"][instance.SubnetId], []string{instance.InstanceId, nodeId})

					}
				}
				if tag["Key"] == "Component" && tag["Value"] == "tidb" {
					if !tidbMap[instance.PrivateIpAddress] {
						tplData.TiDB = append(tplData.TiDB, instance.PrivateIpAddress)
					} else {
						if activeNodesMap["tidb"] == nil {
							activeNodesMap["tidb"] = make(map[string][][]string)
						}
						nodeId := SearchTiDBNode(tidbClusterInfo.Instances, instance.PrivateIpAddress)
						activeNodesMap["tidb"][instance.SubnetId] = append(activeNodesMap["tidb"][instance.SubnetId], []string{instance.InstanceId, nodeId})
					}
				}
				if tag["Key"] == "Component" && tag["Value"] == "tikv" {
					if !tikvMap[instance.PrivateIpAddress] {
						// tplData.TiKV = append(tplData.TiKV, instance.PrivateIpAddress)
						tplTiKVData := TplTiKVData{IPAddress: instance.PrivateIpAddress}
						tplData.TiKV = append(tplData.TiKV, tplTiKVData)
					} else {
						if activeNodesMap["tikv"] == nil {
							activeNodesMap["tikv"] = make(map[string][][]string)
						}
						nodeId := SearchTiDBNode(tidbClusterInfo.Instances, instance.PrivateIpAddress)
						activeNodesMap["tikv"][instance.SubnetId] = append(activeNodesMap["tikv"][instance.SubnetId], []string{instance.InstanceId, nodeId})
					}
				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" {
					if !ticdcMap[instance.PrivateIpAddress] {
						tplData.TiCDC = append(tplData.TiCDC, instance.PrivateIpAddress)
					} else {
						if activeNodesMap["ticdc"] == nil {
							activeNodesMap["ticdc"] = make(map[string][][]string)
						}
						nodeId := SearchTiDBNode(tidbClusterInfo.Instances, instance.PrivateIpAddress)
						activeNodesMap["ticdc"][instance.SubnetId] = append(activeNodesMap["ticdc"][instance.SubnetId], []string{instance.InstanceId, nodeId})
					}
				}
				if tag["Key"] == "Component" && tag["Value"] == "dm" {
					if !dmMap[instance.PrivateIpAddress] {
						tplData.DM = append(tplData.DM, instance.PrivateIpAddress)
					} else {
						if activeNodesMap["dm"] == nil {
							activeNodesMap["dm"] = make(map[string][][]string)
						}
						nodeId := SearchTiDBNode(tidbClusterInfo.Instances, instance.PrivateIpAddress)
						activeNodesMap["dm"][instance.SubnetId] = append(activeNodesMap["dm"][instance.SubnetId], []string{instance.InstanceId, nodeId})
					}
				}
			}
		}
	}

	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplData.String()))

	var nodes2Remove [][]string
	scaledInTiKV := 0
	scaleNode := func(componentName string, nodeInfo []string) error {
		_, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster scale-in %s  -y -N %s --transfer-timeout %d`, clusterName, nodeInfo[1], 200), false, 300*time.Second)
		if err != nil {
			return err
		}
		nodes2Remove = append(nodes2Remove, nodeInfo)

		if componentName == "tikv" {
			scaledInTiKV++
		}
		return nil
	}

	if err = scaleTiDBNode(&scaleNode, &activeNodesMap, "pd", c.awsTopoConfig.TiKV.Count); err != nil {
		return err
	}

	if err = scaleTiDBNode(&scaleNode, &activeNodesMap, "tikv", c.awsTopoConfig.TiKV.Count); err != nil {
		return err
	}

	if err = scaleTiDBNode(&scaleNode, &activeNodesMap, "tidb", c.awsTopoConfig.TiDB.Count); err != nil {
		return err
	}

	if err = scaleTiDBNode(&scaleNode, &activeNodesMap, "ticdc", c.awsTopoConfig.TiDB.Count); err != nil {
		return err
	}

	if len(nodes2Remove) > 0 {
		for cnt := 0; cnt < 100; cnt++ {
			time.Sleep(15 * time.Second)
			tidbClusterInfo, err = getTiDBClusterInfo(workstation, ctx, clusterName)
			if err != nil {
				return err
			}
			numTombstoneNodes := 0
			for _, instance := range tidbClusterInfo.Instances {
				if instance.Status == "Tombstone" {
					numTombstoneNodes++
				}
			}
			if numTombstoneNodes == scaledInTiKV {
				_, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster prune %s -y`, clusterName), false, 600*time.Second)
				if err != nil {
					return err
				}
				for _, node := range nodes2Remove {
					_, _, err = (*c.pexecutor).Execute(ctx, fmt.Sprintf("aws ec2 terminate-instances --instance-ids %s", node[0]), false, 600*time.Second)
					if err != nil {
						return err
					}
				}
				break
			}
		}
	}

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
			// buffer.WriteString(ip)
			buffer.WriteString(ip.IPAddress)
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

	if err := os.WriteFile("/tmp/scale.yaml", buffer.Bytes(), os.FileMode(0644)); err != nil {
		return err
	}
	err = (*workstation).Transfer(ctx, "/tmp/scale.yaml", "/opt/tidb/scale.yaml", false, 0)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster scale-out %s  -y %s`, clusterName, "/opt/tidb/scale.yaml"), false, 300*time.Second)
	if err != nil {
		return err
	}
	return nil
}

func scaleTiDBNode(funcScaleIn *func(componentName string, nodeId []string) error, activeNodesMap *map[string]map[string][][]string, componentName string, nodeCount int) error {
	// tikv/pd/tidb/cdc
	if (*activeNodesMap)[componentName] != nil {
		maxEntity := 0
		for _, nodesBySubnets := range (*activeNodesMap)[componentName] {
			if maxEntity < len(nodesBySubnets) {
				maxEntity = len(nodesBySubnets)
			}
		}
		runNodes := 0
		for idx := 0; idx < maxEntity; idx++ {
			for _, nodesBySubnets := range (*activeNodesMap)[componentName] {
				if idx < len(nodesBySubnets) {
					runNodes++
					// c.awsTopoConfig.TiKV.Count
					if runNodes > nodeCount {
						// To remove the node
						if err := (*funcScaleIn)(componentName, nodesBySubnets[idx]); err != nil {
							return err
						}
					}

				}
			}
		}
	}
	return nil
}

func SearchTiDBNode(clusterComponents []TiDBClusterComponent, host string) string {
	for _, component := range clusterComponents {
		if component.Host == host {
			return component.Id
		}
	}
	return ""
}

// Rollback implements the Task interface
func (c *ScaleTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ScaleTiDB) String() string {
	return fmt.Sprintf("Echo: Scaling TiDB")
}
