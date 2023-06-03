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
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/tidbcloudapi"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	"github.com/luyomo/tidbcloud-sdk-go-v1/pkg/tidbcloud"
)

func (b *Builder) CreateTiDBCloud(tidbCloudConfigs *spec.TiDBCloudConfigs) *Builder {
	b.tasks = append(b.tasks, &CreateTiDBCloud{
		tidbCloudConfigs: tidbCloudConfigs,
	})
	return b
}

func (b *Builder) ListTiDBCloud(projectID uint64, status, clusterType string, tableClusters *[][]string, tableNodes *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListTiDBCloud{
		projectID:     projectID,
		status:        status,
		clusterType:   clusterType,
		tableClusters: tableClusters,
		tableNodes:    tableNodes,
	})
	return b
}

func (b *Builder) DestroyTiDBCloud(workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DestroyTiDBCloud{workstation: workstation})
	return b
}

/* *************************************************************************** */
type BaseTiDBCloud struct {
}

func (c *BaseTiDBCloud) ResourceExist(projectID, clusterName string) (bool, error) {
	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return false, err
	}

	// 01. Look for the cluster
	response, err := client.ListClustersOfProjectWithResponse(context.Background(), projectID, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		return false, err
	}
	for _, item := range response.JSON200.Items {
		if clusterName == *item.Name {
			return true, nil
		}
	}
	return false, nil
}

func (c *CreateTiDBCloud) WaitResourceUnitlAvailable(projectID, clusterName string) error {

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return err
	}

	timeout := time.After(60 * time.Minute)
	d := time.NewTicker(1 * time.Minute)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			response, err := client.ListClustersOfProjectWithResponse(context.Background(), projectID, &tidbcloud.ListClustersOfProjectParams{})
			if err != nil {
				return err
			}
			for _, item := range response.JSON200.Items {
				if clusterName == *item.Name && (*item.Status.ClusterStatus).(string) == "AVAILABLE" {
					return nil
				}
			}

		}
	}
	return nil
}

type CreateTiDBCloud struct {
	BaseTiDBCloud

	// pexecutor *ctxt.Executor
	tidbCloudConfigs *spec.TiDBCloudConfigs
}

// Execute implements the Task interface
func (c *CreateTiDBCloud) Execute(ctx context.Context) error {
	// Get ClusterName from context
	clusterName := ctx.Value("clusterName").(string)

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return err
	}

	clusterExist, err := c.ResourceExist(c.tidbCloudConfigs.TiDBCloudProjectID, clusterName)
	if err != nil {
		return err
	}
	if clusterExist == true {
		return nil
	}

	// 02. Create the cluster
	createClusterJSONRequestBody := tidbcloud.CreateClusterJSONRequestBody{
		CloudProvider: c.tidbCloudConfigs.CloudProvider,
		ClusterType:   c.tidbCloudConfigs.ClusterType,
		Name:          clusterName,
		Region:        c.tidbCloudConfigs.Region,
	}

	createClusterJSONRequestBody.Config.Components = &struct {
		Tidb struct {
			NodeQuantity int32  `json:"node_quantity"`
			NodeSize     string `json:"node_size"`
		} `json:"tidb"`
		Tiflash *struct {
			NodeQuantity   int32  `json:"node_quantity"`
			NodeSize       string `json:"node_size"`
			StorageSizeGib int32  `json:"storage_size_gib"`
		} `json:"tiflash,omitempty"`
		Tikv struct {
			NodeQuantity   int32  `json:"node_quantity"`
			NodeSize       string `json:"node_size"`
			StorageSizeGib int32  `json:"storage_size_gib"`
		} `json:"tikv"`
	}{
		struct {
			NodeQuantity int32  `json:"node_quantity"`
			NodeSize     string `json:"node_size"`
		}{c.tidbCloudConfigs.Components.TiDB.NodeQuantity, c.tidbCloudConfigs.Components.TiDB.NodeSize},
		nil,
		struct {
			NodeQuantity   int32  `json:"node_quantity"`
			NodeSize       string `json:"node_size"`
			StorageSizeGib int32  `json:"storage_size_gib"`
		}{c.tidbCloudConfigs.Components.TiKV.NodeQuantity, c.tidbCloudConfigs.Components.TiKV.NodeSize, c.tidbCloudConfigs.Components.TiKV.StorageSizeGib},
	}

	createClusterJSONRequestBody.Config.IpAccessList = &[]struct {
		Cidr        string  `json:"cidr"`
		Description *string `json:"description,omitempty"`
	}{{c.tidbCloudConfigs.IPAccessList.CIDR, ptr.String(c.tidbCloudConfigs.IPAccessList.Description)}}

	createClusterJSONRequestBody.Config.RootPassword = c.tidbCloudConfigs.Password
	createClusterJSONRequestBody.Config.Port = ptr.Int32(c.tidbCloudConfigs.Port)

	response, err := client.CreateClusterWithResponse(context.Background(), c.tidbCloudConfigs.TiDBCloudProjectID, createClusterJSONRequestBody)
	if err != nil {
		return err
	}

	statusCode := response.StatusCode()
	switch statusCode {
	case 200:
		// fmt.Printf("The common info: <%#v> \n\n\n\n\n\n", response.JSON200)
	case 400:
		return errors.New(fmt.Sprintf("The JSON400 : <%#v> and <%#v> \n\n\n\n\n\n", *response.JSON400.Message, *response.JSON400.Details))

	}

	if err := c.WaitResourceUnitlAvailable(c.tidbCloudConfigs.TiDBCloudProjectID, clusterName); err != nil {
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

	workstation *ws.Workstation
}

// Execute implements the Task interface
func (c *DestroyTiDBCloud) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	fmt.Printf("The cluster name is <%s> \n\n\n", clusterName)
	tidbCloudInfo, err := c.workstation.ReadTiDBCloudDBInfo()
	if err != nil {
		return err
	}
	fmt.Printf("tidb cloud info: %#v \n\n\n", tidbCloudInfo)

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		panic(err)
	}

	response, err := client.ListClustersOfProjectWithResponse(context.Background(), tidbCloudInfo.ProjectID, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		panic(err)
	}

	clusterID := ""
	for _, item := range response.JSON200.Items {
		if *item.Name == clusterName {
			clusterID = item.Id
		}
	}

	if clusterID == "" {
		return nil
	}

	resDelete, err := client.DeleteClusterWithResponse(context.Background(), tidbCloudInfo.ProjectID, clusterID)
	if err != nil {
		panic(err)
	}

	statusCode := resDelete.StatusCode()
	switch statusCode {
	case 200:
		fmt.Printf("Succeeded in deleting the cluster")
		return nil
	case 400:
		fmt.Printf("Failed to delete the cluster: %#v \n", *resDelete.JSON400.Message)
	}

	return errors.New("Failed to destroy tidb cloud cluster")
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
