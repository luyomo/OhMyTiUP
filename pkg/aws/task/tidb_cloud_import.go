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
	"strconv"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/tidbcloudapi"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	"github.com/luyomo/tidbcloud-sdk-go-v1/pkg/tidbcloud"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils/iam"
)

func (b *Builder) CreateTiDBCloudImport(projectId, subClusterType string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &CreateTiDBCloudImport{BaseTiDBCloudImport: BaseTiDBCloudImport{projectId: projectId, subClusterType: subClusterType}})
	return b
}

func (b *Builder) ListTiDBCloudImport(projectID uint64, status, clusterType string, tableClusters *[][]string, tableNodes *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListTiDBCloudImport{
		projectID:     projectID,
		status:        status,
		clusterType:   clusterType,
		tableClusters: tableClusters,
		tableNodes:    tableNodes,
	})
	return b
}

func (b *Builder) DestroyTiDBCloudImport(workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DestroyTiDBCloudImport{workstation: workstation})
	return b
}

/* *************************************************************************** */
type BaseTiDBCloudImport struct {
	BaseWSTask

	projectId      string
	clusterName    string
	subClusterType string
}

func (c *BaseTiDBCloudImport) GetClusterID() (*string, error) {
	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return nil, err
	}

	// 01. Look for the cluster
	response, err := client.ListClustersOfProjectWithResponse(context.Background(), c.projectId, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		return nil, err
	}
	for _, item := range response.JSON200.Items {
		if c.clusterName == *item.Name {
			return &item.Id, nil
		}
	}
	return nil, nil
}

func (c *BaseTiDBCloudImport) ResourceExist() (bool, error) {
	clusterId, err := c.GetClusterID()
	if err != nil {
		return false, err
	}

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return false, err
	}

	// 01. Look for the cluster
	response, err := client.ListImportTasksWithResponse(context.Background(), c.projectId, *clusterId, &tidbcloud.ListImportTasksParams{})
	if err != nil {
		return false, err
	}

	for _, item := range response.JSON200.Items {
		if c.clusterName == *item.Metadata.Name && item.Status.Phase.(string) == "COMPLETED" {
			return true, nil
		}
	}
	return false, nil
}

func (c *CreateTiDBCloudImport) WaitResourceUnitlAvailable() error {

	timeout := time.After(60 * time.Minute)
	d := time.NewTicker(1 * time.Minute)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			resourceExistFlag, err := c.ResourceExist()
			if err != nil {
				return err
			}
			if resourceExistFlag == true {
				return nil
			}

		}
	}
	return nil
}

type CreateTiDBCloudImport struct {
	BaseTiDBCloudImport

	// pexecutor *ctxt.Executor
	// tidbCloudConfigs *spec.TiDBCloudImportConfigs
}

// Executef implements the Task interface
/*
   01. Get bucket arn
   02. Get Path
   03. Role
*/
func (c *CreateTiDBCloudImport) Execute(ctx context.Context) error {
	defer c.takeTimer("Data import to TiDB Cloud")

	// Get ClusterName from context
	c.clusterName = ctx.Value("clusterName").(string)

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return err
	}

	// Skipped the
	// clusterExist, err := c.ResourceExist()
	// if err != nil {
	// 	return err
	// }

	// if clusterExist == true {
	// 	return nil
	// }

	clusterId, err := c.GetClusterID()
	if err != nil {
		return err
	}

	// // 02. Create the cluster

	// Search for the valid S3 backup to import.
	exportTasks, err := awsutils.GetValidBackupS3(c.clusterName)
	if err != nil {
		return err
	}

	if exportTasks == nil {
		return errors.New("No valid snapshot found.")
	}

	if len(*exportTasks) > 1 {
		return errors.New("Multile S3 buckets backup exists")
	}

	if len(*exportTasks) == 0 {
		return errors.New("No buckets backup found")
	}

	backupFile := fmt.Sprintf("s3://%s/%s/%s", *(*exportTasks)[0].S3Bucket, *(*exportTasks)[0].S3Prefix, *(*exportTasks)[0].ExportTaskIdentifier)

	iamapi, err := iam.NewIAMAPI(nil)
	if err != nil {
		return err
	}
	pRoles, err := iamapi.GetRole(c.subClusterType, iam.MakeRoleName(c.clusterName, c.subClusterType))
	if err != nil {
		return err
	}
	if pRoles == nil {
		return errors.New("No role for data import found. ")
	}

	var createImportTaskJSONRequestBody tidbcloud.CreateImportTaskJSONRequestBody
	createImportTaskJSONRequestBody.Name = ptr.String(c.clusterName)
	createImportTaskJSONRequestBody.Spec.Source.Type = "S3"
	createImportTaskJSONRequestBody.Spec.Source.Format.Type = "AURORA_SNAPSHOT"
	createImportTaskJSONRequestBody.Spec.Source.Uri = backupFile
	createImportTaskJSONRequestBody.Spec.Source.AwsAssumeRoleAccess = &struct {
		AssumeRole string `json:"assume_role"`
	}{*(*pRoles)[0].Arn}

	resImport, err := client.CreateImportTaskWithResponse(context.Background(), c.projectId, *clusterId, createImportTaskJSONRequestBody)
	if err != nil {
		return err
	}

	statusCode := resImport.StatusCode()
	switch statusCode {
	case 200:
	case 400:
		return errors.New(fmt.Sprintf("Failed to import data<400>: %s, detail:%#v", *resImport.JSON400.Message, *resImport.JSON400.Details))
	case 403:
		return errors.New(fmt.Sprintf("Failed to import data<403>: %s, detail:%#v", *resImport.JSON403.Message, *resImport.JSON403.Details))
	case 404:
		return errors.New(fmt.Sprintf("Failed to import data<404>: %s, detail:%#v", *resImport.JSON404.Message, *resImport.JSON404.Details))
	case 429:
		return errors.New(fmt.Sprintf("Failed to import data<429>: %s, detail:%#v", *resImport.JSON429.Message, *resImport.JSON429.Details))
	case 500:
		return errors.New(fmt.Sprintf("Failed to import data<500>: %s, detail:%#v", *resImport.JSON500.Message, *resImport.JSON500.Details))
	default:
		return errors.New(fmt.Sprintf("Failed to import data<%d>: %s", statusCode, *resImport))
	}

	if err := c.WaitResourceUnitlAvailable(); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTiDBCloudImport) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiDBCloudImport) String() string {
	return fmt.Sprintf("Echo: Importing S3 parquet data into TiDB Cloud ... ... ")
}

/******************************************************************************/

type DestroyTiDBCloudImport struct {
	pexecutor *ctxt.Executor

	workstation *ws.Workstation
}

// Execute implements the Task interface
func (c *DestroyTiDBCloudImport) Execute(ctx context.Context) error {
	// clusterName := ctx.Value("clusterName").(string)

	tidbCloudInfo, err := c.workstation.ReadTiDBCloudDBInfo()
	if err != nil {
		return err
	}
	fmt.Printf("tidb cloud info: %#v \n", tidbCloudInfo)

	// func (c *BaseTiDBCloudImport) ResourceExist(projectID, clusterName string) (bool, error) {

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTiDBCloudImport) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTiDBCloudImport) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}

type ListTiDBCloudImport struct {
	tableClusters *[][]string
	tableNodes    *[][]string

	projectID   uint64
	clusterType string // Dedicated/Dev
	status      string // PAUSED/AVAILABLE
}

// Execute implements the Task interface
func (c *ListTiDBCloudImport) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	if err := InitClientInstance(); err != nil {
		return err
	}

	_theProjects, err := tidbcloudapi.GetAllProjects()
	if err != nil {
		return err
	}

	var _projects []uint64

	for _, _item := range _theProjects {
		_projects = append(_projects, _item.ID)
	}

	if c.projectID != 0 {
		if containInt64(_projects, c.projectID) == false {
			return errors.New("Invalid project ID")
		}
		_projects = []uint64{c.projectID}
	}

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
func (c *ListTiDBCloudImport) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTiDBCloudImport) String() string {
	return fmt.Sprintf("Echo: List TiDB Cluster ")
}
