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
	// "os"
	"strconv"

	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/tidbcloudapi"
)

type CreateTiDBCloud struct {
	pexecutor *ctxt.Executor
	tidbCloud *spec.TiDBCloud
}

// Execute implements the Task interface
func (c *CreateTiDBCloud) Execute(ctx context.Context) error {
	// Get ClusterName from context
	clusterName := ctx.Value("clusterName").(string)

	if err := InitClientInstance(); err != nil {
		return err
	}

	// Create the cluster
	_url := fmt.Sprintf("%s/api/v1beta/projects/%d/clusters", tidbcloudapi.Host, c.tidbCloud.General.ProjectID)
	payload := tidbcloudapi.CreateClusterReq{
		Name:          clusterName,
		ClusterType:   "DEDICATED",
		CloudProvider: "AWS",
		Region:        c.tidbCloud.General.Region,
		Config: tidbcloudapi.ClusterConfig{
			RootPassword: c.tidbCloud.General.Password,
			Port:         c.tidbCloud.General.Port,
			Components: tidbcloudapi.Components{
				TiDB: &tidbcloudapi.ComponentTiDB{
					NodeSize:     c.tidbCloud.TiDB.NodeSize,
					NodeQuantity: c.tidbCloud.TiDB.Count,
				},
				TiKV: &tidbcloudapi.ComponentTiKV{
					NodeSize:       c.tidbCloud.TiKV.NodeSize,
					NodeQuantity:   c.tidbCloud.TiKV.Count,
					StorageSizeGib: c.tidbCloud.TiKV.Storage,
				},
			},
			IPAccessList: []tidbcloudapi.IPAccess{
				{
					CIDR:        "0.0.0.0/0",
					Description: "Allow Access from Anywhere.",
				},
			},
		},
	}

	var result tidbcloudapi.Cluster
	_, err := tidbcloudapi.DoPOST(_url, payload, &result)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTiDBCloud) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiDBCloud) String() string {
	return fmt.Sprintf("Echo: Create TiDB Cloud ")
}

/******************************************************************************/

type DestroyTiDBCloud struct {
	pexecutor *ctxt.Executor
}

// Execute implements the Task interface
func (c *DestroyTiDBCloud) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	fmt.Printf("The cluster name is <%s> \n\n\n", clusterName)

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTiDBCloud) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTiDBCloud) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}

type ListTiDBCloud struct {
	tableClusters *[][]string
	tableNodes    *[][]string

	projectID   uint64
	clusterType string // Dedicated/Dev
	status      string // PAUSED/AVAILABLE
}

// Execute implements the Task interface
func (c *ListTiDBCloud) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	if err := InitClientInstance(); err != nil {
		return err
	}

	_theProjects, err := tidbcloudapi.GetAllProjects()
	if err != nil {
		return err
	}

	var _projects []uint64
	// fmt.Printf("The projects are %#v \n\n\n", test)
	for _, _item := range _theProjects {
		_projects = append(_projects, _item.ID)
	}

	if c.projectID != 0 {
		if containInt64(_projects, c.projectID) == false {
			return errors.New("Invalid project ID")
		}
		_projects = []uint64{c.projectID}
	}

	// fmt.Printf("Proejct is %#v \n\n\n", c.projectID)
	for _, _projectID := range _projects {
		var (
			url    = fmt.Sprintf("%s/api/v1beta/projects/%d/clusters", tidbcloudapi.Host, _projectID)
			result tidbcloudapi.GetAllClustersResp
		)

		_, err := tidbcloudapi.DoGET(url, nil, &result)
		if err != nil {
			return err
		}

		for _, _item := range result.Items {
			if clusterName != "" && _item.Name != clusterName {
				continue
			}

			if c.status != "ALL" && c.status != _item.Status.ClusterStatus {
				continue
			}

			*(c.tableClusters) = append(*(c.tableClusters), []string{
				strconv.FormatUint(_item.ProjectID, 10),
				_item.Name,
				_item.ClusterType,
				_item.Status.TidbVersion,
				_item.Status.ClusterStatus,
				_item.CloudProvider,
				_item.Region,
				ConvertEpochToString(_item.CreateTimestamp),
			})
			// TiDB
			*(c.tableNodes) = append(*(c.tableNodes), []string{
				strconv.FormatUint(_item.ProjectID, 10),
				_item.Name,
				"TiDB",
				_item.Config.Components.TiDB.NodeSize,
				strconv.FormatInt(int64(_item.Config.Components.TiDB.NodeQuantity), 10),
				"-",
			})

			// TiKV
			*(c.tableNodes) = append(*(c.tableNodes), []string{
				" - ",
				" - ",
				"TiKV",
				_item.Config.Components.TiKV.NodeSize,
				strconv.FormatInt(int64(_item.Config.Components.TiKV.NodeQuantity), 10),
				strconv.FormatInt(int64(_item.Config.Components.TiKV.StorageSizeGib), 10),
			})

			// TiFlash
			if _item.Config.Components.TiFlash != nil {
				*(c.tableNodes) = append(*(c.tableNodes), []string{
					" - ",
					" - ",
					"TiFlash",
					_item.Config.Components.TiFlash.NodeSize,
					strconv.FormatInt(int64(_item.Config.Components.TiFlash.NodeQuantity), 10),
					"-",
				})
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListTiDBCloud) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTiDBCloud) String() string {
	return fmt.Sprintf("Echo: List TiDB Cluster ")
}
