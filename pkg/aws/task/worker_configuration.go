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
	b64 "encoding/base64"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect"
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateWorkerConfiguration() *Builder {
	b.tasks = append(b.tasks, &CreateWorkerConfiguration{})
	return b
}

/******************************************************************************/

type WorkerConfigurationInfo struct {
	Name                   *string
	WorkerConfigurationArn *string
	LatestRevision         int64
}

type WorkerConfigurationInfos struct {
	BaseResourceInfo
}

func (d *WorkerConfigurationInfos) Append(workerConfiguration *types.WorkerConfigurationSummary) {
	d.Data = append(d.Data, WorkerConfigurationInfo{
		Name:                   workerConfiguration.Name,
		WorkerConfigurationArn: workerConfiguration.WorkerConfigurationArn,
		LatestRevision:         (*workerConfiguration.LatestRevision).Revision,
	})
}

func (d *WorkerConfigurationInfos) IsEmpty() bool {
	if len((*d).Data) == 0 {
		return false
	}

	return true
}

func (d *WorkerConfigurationInfos) GetWorkerConfigurationARN() (*string, error) {
	if len((*d).Data) == 0 {
		return nil, errors.New("No worker configuration found")
	}

	if len((*d).Data) > 1 {
		return nil, errors.New("Duplicate worker configurations found")
	}

	return (d.Data[0]).(WorkerConfigurationInfo).WorkerConfigurationArn, nil
}

func (d *WorkerConfigurationInfos) ToPrintTable() *[][]string {
	tableWorkerConfiguration := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(WorkerConfigurationInfo)
		tableWorkerConfiguration = append(tableWorkerConfiguration, []string{
			*_entry.Name,
		})
	}
	return &tableWorkerConfiguration
}

type BaseWorkerConfiguration struct {
	pexecutor                *ctxt.Executor
	WorkerConfigurationInfos *WorkerConfigurationInfos
	/* awsWorkerConfigurationTopoConfigs *spec.AwsWorkerConfigurationTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client         *kafkaconnect.Client // Replace the example to specific service
	clusterName    string               // It's initialized from init() function
	clusterType    string               // It's initialized from init() function
	subClusterType string               // It's set from initializtion from caller
}

func (b *BaseWorkerConfiguration) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = kafkaconnect.NewFromConfig(cfg) // Replace the example to specific service

	if b.WorkerConfigurationInfos == nil {
		b.WorkerConfigurationInfos = &WorkerConfigurationInfos{}
	}

	return nil
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseWorkerConfiguration) ClusterExist(checkAvailableState bool) (bool, error) {
	if err := b.ReadWorkerConfigurationInfo(); err != nil {
		return false, err
	}

	return b.WorkerConfigurationInfos.IsEmpty(), nil
}

func (b *BaseWorkerConfiguration) ReadWorkerConfigurationInfo() error {
	resp, err := b.client.ListWorkerConfigurations(context.TODO(), &kafkaconnect.ListWorkerConfigurationsInput{})
	if err != nil {
		return err
	}

	for _, workerConfiguration := range resp.WorkerConfigurations {
		if *workerConfiguration.Name == b.clusterName {
			b.WorkerConfigurationInfos.Append(&workerConfiguration)
		}
	}

	return nil
}

type CreateWorkerConfiguration struct {
	BaseWorkerConfiguration

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateWorkerConfiguration) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** CreateWorkerConfigurationCluster ****** \n\n\n")

	clusterExistFlag, err := c.ClusterExist(false)
	if err != nil {
		return err
	}
	if clusterExistFlag == false {
		if _, err = c.client.CreateWorkerConfiguration(context.TODO(), &kafkaconnect.CreateWorkerConfigurationInput{
			Name: aws.String(c.clusterName),
			PropertiesFileContent: aws.String(b64.StdEncoding.EncodeToString([]byte(`key.converter=com.amazonaws.services.schemaregistry.kafkaconnect.AWSKafkaAvroConverter
key.converter.schemas.enable=false
value.converter=com.amazonaws.services.schemaregistry.kafkaconnect.AWSKafkaAvroConverter
value.converter.schemas.enable=false`))),
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateWorkerConfiguration) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateWorkerConfiguration) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

type DestroyWorkerConfiguration struct {
	BaseWorkerConfiguration
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyWorkerConfiguration) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyWorkerConfigurationCluster ****** \n\n\n")

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
func (c *DestroyWorkerConfiguration) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyWorkerConfiguration) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListWorkerConfiguration struct {
	BaseWorkerConfiguration
}

// Execute implements the Task interface
func (c *ListWorkerConfiguration) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListWorkerConfigurationCluster ****** \n\n\n")

	if err := c.ReadWorkerConfigurationInfo(); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListWorkerConfiguration) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListWorkerConfiguration) String() string {
	return fmt.Sprintf("Echo: List WorkerConfiguration ")
}
