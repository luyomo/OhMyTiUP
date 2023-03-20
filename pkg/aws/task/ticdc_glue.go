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
	"fmt"

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

type TiCDCGlueInfo struct {
	ClusterName string
}

type TiCDCGlueInfos struct {
	BaseResourceInfo
}

func (d *TiCDCGlueInfos) Append( /*cluster *types.Cluster*/ ) {
	(*d).Data = append((*d).Data, TiCDCGlueInfo{
		ClusterName: "TiCDCGlue",
	})
}

func (d *TiCDCGlueInfos) ToPrintTable() *[][]string {
	tableTiCDCGlue := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(TiCDCGlueInfo)
		tableTiCDCGlue = append(tableTiCDCGlue, []string{
			_entry.ClusterName,
		})
	}
	return &tableTiCDCGlue
}

type BaseTiCDCGlue struct {
	BaseTask

	// pexecutor      *ctxt.Executor
	TiCDCGlueInfos *TiCDCGlueInfos
	/* awsTiCDCGlueTopoConfigs *spec.AwsTiCDCGlueTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	/* client  *example.Client */ // Replace the example to specific service
	// clusterName                   string // It's initialized from init() function
	// clusterType                   string // It's initialized from init() function
	subClusterType string // It's set from initializtion from caller
}

func (b *BaseTiCDCGlue) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	log.Infof(fmt.Sprintf("config: %#v", cfg))

	/* b.client = example.NewFromConfig(cfg) */ // Replace the example to specific service

	return nil
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseTiCDCGlue) ClusterExist(checkAvailableState bool) (bool, error) {
	return false, nil
}

func (b *BaseTiCDCGlue) ReadTiCDCGlueInfo(ctx context.Context) error {
	return nil
}

type CreateTiCDCGlue struct {
	BaseTiCDCGlue

	// wsExe       *ctxt.Executor
	// clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiCDCGlue) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** CreateTiCDCGlueCluster ****** \n\n\n")

	if _, _, err := (*c.wsExe).Execute(context.Background(), "rm -f cdc.x86_64.zip && wget https://github.com/luyomo/tiflow-glue/releases/download/glue/cdc.x86_64.zip", false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), "rm -f cdc && unzip cdc.x86_64.zip", false); err != nil {
		return err
	}

	cdcInstances, err := c.getTiDBComponent("cdc")
	if err != nil {
		return err
	}
	fmt.Printf("The cdc data is <%#v> \n\n\n\n\n\n", cdcInstances)

	for _, instance := range *cdcInstances {
		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("/home/admin/.tiup/bin/tiup cluster stop -y %s --node %s", c.clusterName, instance.ID), false); err != nil {
			return err
		}

		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("scp -o  StrictHostKeyChecking=no cdc %s:%s/bin", instance.Host, instance.DeployDir), false); err != nil {
			return err
		}

		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("/home/admin/.tiup/bin/tiup cluster start -y %s --node %s", c.clusterName, instance.ID), false); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTiCDCGlue) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiCDCGlue) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}
