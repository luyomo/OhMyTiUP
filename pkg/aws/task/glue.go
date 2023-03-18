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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

type GlueSchemaRegistryInfo struct {
	ClusterName string
}

type GlueSchemaRegistryInfos struct {
	BaseResourceInfo
}

func (d *GlueSchemaRegistryInfos) Append( /*cluster *types.Cluster*/ ) {
	(*d).Data = append((*d).Data, GlueSchemaRegistryInfo{
		ClusterName: "GlueSchemaRegistry",
	})
}

func (d *GlueSchemaRegistryInfos) ToPrintTable() *[][]string {
	tableGlueSchemaRegistry := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(GlueSchemaRegistryInfo)
		tableGlueSchemaRegistry = append(tableGlueSchemaRegistry, []string{
			_entry.ClusterName,
		})
	}
	return &tableGlueSchemaRegistry
}

type BaseGlueSchemaRegistryCluster struct {
	pexecutor               *ctxt.Executor
	GlueSchemaRegistryInfos *GlueSchemaRegistryInfos
	/* awsGlueSchemaRegistryTopoConfigs *spec.AwsGlueSchemaRegistryTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client         *glue.Client // Replace the example to specific service
	clusterName    string       // It's initialized from init() function
	clusterType    string       // It's initialized from init() function
	subClusterType string       // It's set from initializtion from caller
}

func (b *BaseGlueSchemaRegistryCluster) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = glue.NewFromConfig(cfg)

	return nil
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseGlueSchemaRegistryCluster) ClusterExist(checkAvailableState bool) (bool, error) {
	// Call the ListRegistries API operation to get a list of all schema registries.
	resp, err := b.client.ListRegistries(context.TODO(), &glue.ListRegistriesInput{})
	if err != nil {
		return false, err
	}

	// Loop through the schema registries and filter them by name.
	for _, registry := range resp.Registries {
		if *registry.RegistryName == b.clusterName {
			return true, nil
		}
	}

	return false, nil
}

func (b *BaseGlueSchemaRegistryCluster) getClusterArn() (*string, error) {
	// Call the ListRegistries API operation to get a list of all schema registries.
	resp, err := b.client.ListRegistries(context.TODO(), &glue.ListRegistriesInput{})
	if err != nil {
		return nil, err
	}

	// Loop through the schema registries and filter them by name.
	for _, registry := range resp.Registries {
		if *registry.RegistryName == b.clusterName {
			return registry.RegistryArn, nil
		}
	}

	return nil, nil
}

func (b *BaseGlueSchemaRegistryCluster) ReadGlueSchemaRegistryInfo(ctx context.Context) error {
	return nil
}

type CreateGlueSchemaRegistryCluster struct {
	BaseGlueSchemaRegistryCluster

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateGlueSchemaRegistryCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	log.Infof("***** CreateGlueSchemaRegistryCluster ****** \n\n\n\n")

	clusterExistFlag, err := c.ClusterExist(false)
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// Call the CreateRegistry API operation to create the schema registry.
		if _, err := c.client.CreateRegistry(context.TODO(), &glue.CreateRegistryInput{
			RegistryName: aws.String(c.clusterName),
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateGlueSchemaRegistryCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateGlueSchemaRegistryCluster) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

type DestroyGlueSchemaRegistryCluster struct {
	BaseGlueSchemaRegistryCluster
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyGlueSchemaRegistryCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	registryArn, err := c.getClusterArn()
	if err != nil {
		return err
	}

	if registryArn != nil {
		_, err = c.client.DeleteRegistry(context.TODO(), &glue.DeleteRegistryInput{
			RegistryId: &types.RegistryId{
				RegistryArn: registryArn,
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyGlueSchemaRegistryCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyGlueSchemaRegistryCluster) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListGlueSchemaRegistryCluster struct {
	BaseGlueSchemaRegistryCluster
}

// Execute implements the Task interface
func (c *ListGlueSchemaRegistryCluster) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	if err := c.ReadGlueSchemaRegistryInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListGlueSchemaRegistryCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListGlueSchemaRegistryCluster) String() string {
	return fmt.Sprintf("Echo: List GlueSchemaRegistry ")
}
