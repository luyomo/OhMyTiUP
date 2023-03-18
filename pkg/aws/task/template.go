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
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

type ExampleInfo struct {
	ClusterName string
}

type ExampleInfos struct {
	BaseResourceInfo
}

func (d *ExampleInfos) Append( /*cluster *types.Cluster*/ ) {
	(*d).Data = append((*d).Data, ExampleInfo{
		ClusterName: "Example",
	})
}

func (d *ExampleInfos) ToPrintTable() *[][]string {
	tableExample := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(ExampleInfo)
		tableExample = append(tableExample, []string{
			_entry.ClusterName,
		})
	}
	return &tableExample
}

type BaseExampleCluster struct {
	pexecutor    *ctxt.Executor
	ExampleInfos *ExampleInfos
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	/* client  *example.Client */ // Replace the example to specific service
	clusterName                   string // It's initialized from init() function
	clusterType                   string // It's initialized from init() function
	subClusterType                string // It's set from initializtion from caller
}

func (b *BaseExampleCluster) init(ctx context.Context) error {
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
func (b *BaseExampleCluster) ClusterExist(checkAvailableState bool) (bool, error) {
	return false, nil
}

func (b *BaseExampleCluster) ReadExampleInfo(ctx context.Context) error {
	return nil
}

type CreateExampleCluster struct {
	BaseExampleCluster

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateExampleCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** CreateExampleCluster ****** \n\n\n")

	clusterExistFlag, err := c.ClusterExist(false)
	if err != nil {
		return err
	}
	if clusterExistFlag == false {
		// TODO: Create the cluster

		// TODO: Check cluster status until expected status
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateExampleCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateExampleCluster) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

type DestroyExampleCluster struct {
	BaseExampleCluster
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyExampleCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyExampleCluster ****** \n\n\n")

	clusterExistFlag, err := c.ClusterExist(false)
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		// Destroy the cluster
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyExampleCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyExampleCluster) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListExampleCluster struct {
	BaseExampleCluster
}

// Execute implements the Task interface
func (c *ListExampleCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListExampleCluster ****** \n\n\n")

	if err := c.ReadExampleInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListExampleCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListExampleCluster) String() string {
	return fmt.Sprintf("Echo: List Example ")
}
