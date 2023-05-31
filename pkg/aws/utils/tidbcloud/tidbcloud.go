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

package tidbcloud

import (
	"context"
	// "encoding/json"
	// "errors"
	"fmt"
	"github.com/luyomo/tidbcloud-sdk-go-v1/pkg/tidbcloud"
	// "os"
	// "path"
	// "sort"
	// "strings"
	// "text/template"
	// "time"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
	// "go.uber.org/zap"
)

type TiDBCloudAPI struct {
	client      *tidbcloud.ClientWithResponses
	projectId   string
	clusterId   string
	clusterName string

	mapArgs *map[string]string
}

func NewTiDBCloudAPI(projectId, clusterName string, mapArgs *map[string]string) (*TiDBCloudAPI, error) {
	tidbCloudApi := TiDBCloudAPI{
		projectId:   projectId,
		clusterName: clusterName,
	}

	if mapArgs != nil {
		tidbCloudApi.mapArgs = mapArgs
	}

	client, err := tidbcloud.NewDigestClientWithResponses()
	if err != nil {
		return nil, err
	}

	tidbCloudApi.client = client
	if err := tidbCloudApi.setClusterId(); err != nil {
		return nil, err
	}

	return &tidbCloudApi, nil
}

func (t *TiDBCloudAPI) setClusterId() error {
	clusterId, err := t.GetClusterId()
	if err != nil {
		return err
	}
	if clusterId != nil {
		t.clusterId = *clusterId
	}
	return nil
}

func (t *TiDBCloudAPI) GetClusterId() (*string, error) {
	response, err := t.client.ListClustersOfProjectWithResponse(context.Background(), t.projectId, &tidbcloud.ListClustersOfProjectParams{})
	if err != nil {
		return nil, err
	}

	for _, item := range response.JSON200.Items {
		if *item.Name == t.clusterName && (*item.Status.ClusterStatus).(string) == "AVAILABLE" {
			return &item.Id, nil
		}
	}
	return nil, nil
}

func (t *TiDBCloudAPI) GetImportTaskRoleInfo() (*string, *string, error) {
	fmt.Printf("Starting to get import task role info ... ... \n\n\n")

	response, err := t.client.GetImportTaskRoleInfoWithResponse(context.Background(), t.projectId, t.clusterId)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("The response is : %s and %s  \n\n\n", response.JSON200.AwsImportRole.AccountId, response.JSON200.AwsImportRole.ExternalId)
	// for _, item := range response.JSON200.Items {
	// 	fmt.Printf("The item: %#v \n\n\n", item)
	// 	// if c.clusterName == *item.Name {
	// 	// 	return &item.Id, nil
	// 	// }
	// }
	return &response.JSON200.AwsImportRole.AccountId, &response.JSON200.AwsImportRole.ExternalId, nil
}
