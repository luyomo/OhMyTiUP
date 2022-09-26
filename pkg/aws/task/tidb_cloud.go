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
	// "errors"
	"fmt"
	"os"
	// "strconv"
	// "time"

	// "github.com/aws/aws-sdk-go-v2/aws"
	// "github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	// "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/luyomo/tisample/pkg/aws/spec"
	// "github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/ctxt"
	// "io/ioutil"
	"github.com/luyomo/tisample/pkg/tidbcloudapi"
)

type CreateTiDBCloud struct {
	pexecutor *ctxt.Executor
	tidbCloud *spec.TiDBCloud
}

// Execute implements the Task interface
func (c *CreateTiDBCloud) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	var (
		publicKey  = os.Getenv("TIDBCLOUD_PUBLIC_KEY")
		privateKey = os.Getenv("TIDBCLOUD_PRIVATE_KEY")
	)
	if publicKey == "" || privateKey == "" {
		fmt.Printf("Please set TIDBCLOUD_PUBLIC_KEY(%s), TIDBCLOUD_PRIVATE_KEY(%s) in environment variable first\n", publicKey, privateKey)
		return nil
	}

	err := tidbcloudapi.InitClient(publicKey, privateKey)
	if err != nil {
		fmt.Printf("Failed to init HTTP client\n")
		return err
	}

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

	var result tidbcloudapi.GetClusterResp
	_, err = tidbcloudapi.DoPOST(_url, payload, &result)
	if err != nil {
		return err
	}
	fmt.Printf("The res is <%#v> \n", result)

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
	pexecutor      *ctxt.Executor
	tableTiDBCloud *[][]string
}

// Execute implements the Task interface
func (c *ListTiDBCloud) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	fmt.Printf("The cluster name is <%s> \n\n\n", clusterName)

	return nil
}

// Rollback implements the Task interface
func (c *ListTiDBCloud) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTiDBCloud) String() string {
	return fmt.Sprintf("Echo: List Aurora ")
}
