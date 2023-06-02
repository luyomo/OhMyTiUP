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

package workstation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	// "gopkg.in/yaml.v3"
	// "io/ioutil"
	// "os"
	// "path/filepath"
	// "go.uber.org/zap"
	// "github.com/aws/aws-sdk-go-v2/aws"
	// "github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "github.com/luyomo/OhMyTiUP/pkg/executor"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
)

/* ***************************************************************************** */
/*
   Target: Cluster info
*/
func (w *Workstation) GetDMCluster(clusterName string) (*map[string]interface{}, error) {
	ctx := context.Background()

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dm list --format json 2>/dev/null", w.tiupCmdPath), false)
	if err != nil {
		return nil, err
	}

	var dmListInfo map[string][]interface{}
	if err := json.Unmarshal(stdout, &dmListInfo); err != nil {
		return nil, err
	}

	// Check whether the cluster exists
	for _, _cluster := range dmListInfo["clusters"] {
		if _cluster.(map[string]interface{})["name"] == clusterName {
			_, ok := _cluster.(map[string]interface{})
			if ok {
				stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dm display %s --format json 2>/dev/null", w.tiupCmdPath, clusterName), false)
				if err != nil {
					return nil, err
				}
				var dmInfo map[string]interface{}
				if err := json.Unmarshal(stdout, &dmInfo); err != nil {
					return nil, err
				}
				fmt.Printf("The cluster info: <%#v> \n\n\n", dmInfo)

				return &dmInfo, nil
			}
		}
	}
	return nil, nil
}

func (w *Workstation) DeployDMCluster(clusterName, clusterVersion string, mapDBInstances *map[string][]string) error {
	ctx := context.Background()

	dmCluster, err := w.GetDMCluster(clusterName)
	if err != nil {
		return err
	}

	if dmCluster == nil {
		fmt.Printf("Starting to deploy dm \n\n\n")
		if _, _, err := (*w.executor).Execute(ctx, "mkdir -p /opt/tidb", true); err != nil {
			return err
		}

		if err := (*w.executor).TransferTemplate(ctx, "templates/config/dm-cluster.yml.tpl", "/opt/tidb/dm-cluster.yml", "0644", *mapDBInstances, true, 0); err != nil {
			return err
		}

		_, _, err = (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dm deploy %s %s %s -y", w.tiupCmdPath, clusterName, clusterVersion, "/opt/tidb/dm-cluster.yml"), false)
		if err != nil {
			return err
		}

		_, _, err = (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dm start %s -y", w.tiupCmdPath, clusterName), false, 5*time.Minute)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Workstation) GetDMMasterAddr(clusterName string) (*[]string, error) {
	dmCluster, err := w.GetDMCluster(clusterName)
	if err != nil {
		return nil, err
	}
	if dmCluster == nil {
		return nil, nil
	}

	var masterIPs []string
	for _, dmCluster := range ((*dmCluster)["instances"]).([]interface{}) {
		_dmCluster := dmCluster.(map[string]interface{})
		if _dmCluster["role"].(string) == "dm-master" {
			masterIPs = append(masterIPs, _dmCluster["id"].(string))
		}
	}

	if len(masterIPs) == 0 {
		return nil, nil
	} else {
		return &masterIPs, nil
	}
}

/* *************************************************************************** */
/*
   Target: DM Source
*/
func (w *Workstation) GetDMSource(clusterName string) (*map[string]interface{}, error) {
	ctx := context.Background()

	pDMMasterAddr, err := w.GetDMMasterAddr(clusterName)
	if err != nil {
		return nil, err
	}
	if pDMMasterAddr == nil {
		return nil, errors.New("No DM cluster found.")
	}

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dmctl --master-addr %s operate-source show", w.tiupCmdPath, (*pDMMasterAddr)[0]), false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return nil, err
	}

	var dmSourceInfo map[string]interface{}
	if err := json.Unmarshal(stdout, &dmSourceInfo); err != nil {
		return nil, err
	}

	for _, source := range dmSourceInfo["sources"].([]interface{}) {
		if (source.(map[string]interface{}))["source"].(string) == clusterName {
			_source := source.(map[string]interface{})
			return &_source, nil
		}
	}
	return nil, nil
}

func (w *Workstation) DeployDMSource(clusterName string, mapDBConnInfo *map[string]interface{}) error {
	ctx := context.Background()

	dmSource, err := w.GetDMSource(clusterName)
	if err != nil {
		return err
	}
	if dmSource != nil {
		return nil
	}

	pDMMasterAddr, err := w.GetDMMasterAddr(clusterName)
	if err != nil {
		return err
	}
	if pDMMasterAddr == nil {
		return errors.New("No DM cluster found.")
	}

	fmt.Printf("The source connection info: %#v \n\n\n", mapDBConnInfo)
	if err := (*w.executor).TransferTemplate(ctx, "templates/config/dm-source.yml.tpl", "/opt/tidb/dm-source.yml", "0644", mapDBConnInfo, true, 0); err != nil {
		return err
	}

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dmctl --master-addr %s operate-source create /opt/tidb/dm-source.yml", w.tiupCmdPath, (*pDMMasterAddr)[0]), false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}
	return nil
}

/* *************************************************************************** */
/*
   Target: DM Task
*/

func (w *Workstation) GetDMTask(clusterName string) (*map[string]interface{}, error) {
	ctx := context.Background()

	pDMMasterAddr, err := w.GetDMMasterAddr(clusterName)
	if err != nil {
		return nil, err
	}
	if pDMMasterAddr == nil {
		return nil, errors.New("No DM cluster found.")
	}

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dmctl --master-addr %s query-status", w.tiupCmdPath, (*pDMMasterAddr)[0]), false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return nil, err
	}

	var dmTaskInfo map[string]interface{}
	if err := json.Unmarshal(stdout, &dmTaskInfo); err != nil {
		return nil, err
	}

	for _, task := range dmTaskInfo["tasks"].([]interface{}) {
		if (task.(map[string]interface{}))["taskName"].(string) == clusterName {
			_task := task.(map[string]interface{})
			return &_task, nil
		}
	}
	return nil, nil
}

func (w *Workstation) DeployDMTask(clusterName string, mapDBConnInfo *map[string]interface{}) error {
	ctx := context.Background()

	fmt.Printf("Starting the DeployDMTsk \n\n\n")

	pDMMasterAddr, err := w.GetDMMasterAddr(clusterName)
	if err != nil {
		return err
	}
	if pDMMasterAddr == nil {
		return errors.New("No DM cluster found.")
	}

	task, err := w.GetDMTask(clusterName)
	if err != nil {
		return err
	}
	fmt.Printf("The task is: <%#v> \n\n\n", task)
	if task != nil {
		return nil
	}

	return errors.New("Stopped here")

	(*mapDBConnInfo)["Databases"] = strings.Split((*mapDBConnInfo)["Databases"].(string), ",")

	if err := (*w.executor).TransferTemplate(ctx, "templates/config/dm-task.yml.tpl", "/opt/tidb/dm-task.yml", "0644", mapDBConnInfo, true, 0); err != nil {
		return err
	}

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("%s/tiup dmctl --master-addr %s start-task /opt/tidb/dm-task.yml", w.tiupCmdPath, (*pDMMasterAddr)[0]), false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}
	return nil
}

func (w *Workstation) SyncDiffInspector(clusterName, databases string) error {
	ctx := context.Background()

	pDMMasterAddr, err := w.GetDMMasterAddr(clusterName)
	if err != nil {
		return err
	}
	if pDMMasterAddr == nil {
		return errors.New("No DM cluster found.")
	}

	mapArgs := make(map[string]interface{})
	mapArgs["DMMasterAddr"] = (*pDMMasterAddr)[0]
	mapArgs["TaskName"] = clusterName
	mapArgs["Databases"] = strings.Split(databases, ",")

	if err := (*w.executor).TransferTemplate(ctx, "templates/config/sync_diff_inspector.toml.tpl", "/opt/tidb/sync_diff_inspector.toml", "0644", mapArgs, true, 0); err != nil {
		return err
	}

	stdout, _, err := (*w.executor).Execute(ctx, "rm -rf /tmp/output/config/*", false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*w.executor).Execute(ctx, fmt.Sprintf("%s/sync_diff_inspector --config /opt/tidb/sync_diff_inspector.toml", w.tiupCmdPath), false, 1*time.Minute)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	return nil
}
