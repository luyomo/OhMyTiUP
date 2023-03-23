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
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect"
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateMskConnect(wsExe *ctxt.Executor, createMskConnectInput *CreateMskConnectInput /* redshiftDBInfo *ws.RedshiftDBInfo, mskEndpoints *string, glueSchemaRegistry, topicName, tableName string*/) *Builder {
	b.tasks = append(b.tasks, &CreateMskConnect{
		BaseMskConnect:        BaseMskConnect{BaseTask: BaseTask{wsExe: wsExe}},
		createMskConnectInput: createMskConnectInput,
		// mskEndpoints:       mskEndpoints,
		// redshiftDBInfo:     redshiftDBInfo,
		// glueSchemaRegistry: glueSchemaRegistry,
		// topicName:          topicName,
		// tableName:          tableName,
	})
	return b
}

/******************************************************************************/

type MskConnectInfo struct {
	ClusterName string
}

type MskConnectInfos struct {
	BaseResourceInfo
}

func (d *MskConnectInfos) Append( /*cluster *types.Cluster*/ ) {
	(*d).Data = append((*d).Data, MskConnectInfo{
		ClusterName: "MskConnect",
	})
}

func (d *MskConnectInfos) ToPrintTable() *[][]string {
	tableMskConnect := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(MskConnectInfo)
		tableMskConnect = append(tableMskConnect, []string{
			_entry.ClusterName,
		})
	}
	return &tableMskConnect
}

type BaseMskConnect struct {
	BaseTask

	MskConnectInfos *MskConnectInfos
	/* awsMskConnectTopoConfigs *spec.AwsMskConnectTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *kafkaconnect.Client // Replace the example to specific service

	subClusterType string // It's set from initializtion from caller
}

func (b *BaseMskConnect) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	log.Infof(fmt.Sprintf("config: %#v", cfg))

	b.client = kafkaconnect.NewFromConfig(cfg) // Replace the example to specific service

	return nil
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseMskConnect) ClusterExist(checkAvailableState bool) (bool, error) {
	return false, nil
}

func (b *BaseMskConnect) ReadMskConnectInfo(ctx context.Context) error {
	return nil
}

type CreateMskConnectInput struct {
	RedshiftDBInfo     *ws.RedshiftDBInfo // Redshift DB info
	MskEndpoints       *string            // AWS MSK Endpoints
	GlueSchemaRegistry string             // Glue schema registry
	Region             string             // Glue schema registry's region
	TopicName          string             // topic name to be consumed
	TableName          string             // table name to sync

}

type CreateMskConnect struct {
	BaseMskConnect

	clusterInfo *ClusterInfo

	createMskConnectInput *CreateMskConnectInput

	// redshiftDBInfo     *ws.RedshiftDBInfo
	// mskEndpoints       *string
	// glueSchemaRegistry string
	// topicName          string
	// tableName          string
}

// Execute implements the Task interface
func (c *CreateMskConnect) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** CreateMskConnectCluster ****** \n\n\n")

	clusterExistFlag, err := c.ClusterExist(false)
	if err != nil {
		return err
	}
	if clusterExistFlag == false {
		fmt.Printf("****** Starting to create the cluster ***** \n\n\n\n\n\n")
		// TODO: Create the cluster

		// 1. Get redshift endpoint(yaml file)
		// 2. AWS MSK endpoint
		// 3. topic name to fetch
		// 4. table
		// 5. glue schema registry name

		fmt.Printf("The redshift db info: <%#v> \n\n\n\n\n\n", c.createMskConnectInput.RedshiftDBInfo)
		connectConfiguration := make(map[string]string)

		// 1. Get subnets
		// 2. Get security group
		// 3. Get custom plugin arn
		var mskConnectPluginInfos MSKConnectPluginInfos
		listMSKConnectPlugin := &ListMSKConnectPlugin{BaseMSKConnectPlugin: BaseMSKConnectPlugin{pexecutor: c.pexecutor, MSKConnectPluginInfos: &mskConnectPluginInfos, clusterName: c.clusterName}}
		if err := listMSKConnectPlugin.Execute(ctx); err != nil {
			return err
		}
		pluginArn, err := mskConnectPluginInfos.GetPluginArn()
		if err != nil {
			return err
		}
		fmt.Printf("The connect plugin: <%#v>\n\n\n\n ", *pluginArn)

		// 4. Execution role
		// 5. worker configuration arn
		return nil

		connectConfiguration["connector.class"] = "io.confluent.connect.aws.redshift.RedshiftSinkConnector"
		connectConfiguration["tasks.max"] = "1"
		connectConfiguration["confluent.topic.bootstrap.servers"] = *c.createMskConnectInput.MskEndpoints
		connectConfiguration["name"] = c.clusterName
		connectConfiguration["topics"] = c.createMskConnectInput.TopicName
		connectConfiguration["aws.redshift.domain"] = c.createMskConnectInput.RedshiftDBInfo.Host
		connectConfiguration["aws.redshift.port"] = fmt.Sprintf("%d", c.createMskConnectInput.RedshiftDBInfo.Port)
		connectConfiguration["aws.redshift.database"] = c.createMskConnectInput.RedshiftDBInfo.DBName
		connectConfiguration["aws.redshift.user"] = c.createMskConnectInput.RedshiftDBInfo.UserName
		connectConfiguration["aws.redshift.password"] = c.createMskConnectInput.RedshiftDBInfo.Password
		connectConfiguration["table.name.format"] = c.createMskConnectInput.TableName
		connectConfiguration["insert.mode"] = "insert"
		connectConfiguration["delete.enabled"] = "true"
		connectConfiguration["pk.mode"] = "record_key"
		connectConfiguration["auto.create"] = "true"
		connectConfiguration["key.converter"] = "com.amazonaws.services.schemaregistry.kafkaconnect.AWSKafkaAvroConverter"
		connectConfiguration["key.converter.schemas.enable"] = "false"
		connectConfiguration["key.converter.region"] = c.createMskConnectInput.Region
		connectConfiguration["key.converter.schemaAutoRegistrationEnabled"] = "true"
		connectConfiguration["key.converter.avroRecordType"] = "GENERIC_RECORD"
		connectConfiguration["key.converter.registry.name"] = c.createMskConnectInput.GlueSchemaRegistry
		connectConfiguration["value.converter"] = "com.amazonaws.services.schemaregistry.kafkaconnect.AWSKafkaAvroConverter"
		connectConfiguration["value.converter.schemas.enable"] = "false"
		connectConfiguration["value.converter.region"] = c.createMskConnectInput.Region
		connectConfiguration["value.converter.schemaAutoRegistrationEnabled"] = "true"
		connectConfiguration["value.converter.avroRecordType"] = "GENERIC_RECORD"
		connectConfiguration["value.converter.registry.name"] = c.createMskConnectInput.GlueSchemaRegistry

		_, err = c.client.CreateConnector(context.TODO(), &kafkaconnect.CreateConnectorInput{
			Capacity: &types.Capacity{
				ProvisionedCapacity: &types.ProvisionedCapacity{
					McuCount:    1,
					WorkerCount: 1,
				},
			},
			ConnectorConfiguration: connectConfiguration,
			ConnectorName:          aws.String(c.clusterName),
			KafkaCluster: &types.KafkaCluster{
				ApacheKafkaCluster: &types.ApacheKafkaCluster{
					BootstrapServers: c.createMskConnectInput.MskEndpoints,
					Vpc: &types.Vpc{
						Subnets:        []string{"subnet-0fb529d0d16dbe33b", "subnet-03974f66caf5253f0", "subnet-0fed73a70606d4af3"},
						SecurityGroups: []string{"sg-0789992f83fbc7353"},
					},
				},
			},
			KafkaClusterClientAuthentication: &types.KafkaClusterClientAuthentication{
				AuthenticationType: types.KafkaClusterClientAuthenticationTypeNone,
			},
			KafkaClusterEncryptionInTransit: &types.KafkaClusterEncryptionInTransit{
				EncryptionType: types.KafkaClusterEncryptionInTransitTypePlaintext,
			},
			KafkaConnectVersion: aws.String("2.7.1"),
			Plugins: []types.Plugin{
				types.Plugin{
					CustomPlugin: &types.CustomPlugin{
						CustomPluginArn: aws.String("arn:aws:kafkaconnect:us-east-1:729581434105:custom-plugin/aws-msk-redshift-sink-plugin/9eb7f4c7-5b37-4e5c-8e95-39d9fffa6c3a-2"),
						Revision:        1,
					},
				},
			},
			ServiceExecutionRoleArn: aws.String("arn:aws:iam::729581434105:role/tidb2es-role"),
			WorkerConfiguration: &types.WorkerConfiguration{
				WorkerConfigurationArn: aws.String("arn:aws:kafkaconnect:us-east-1:729581434105:worker-configuration/test001/0fbbe690-6f20-4555-bc2b-09c736b7cf02-2"),
				Revision:               1,
			},
		})
		if err != nil {
			return err
		}

		// TODO: Check cluster status until expected status
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateMskConnect) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateMskConnect) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

type DestroyMskConnect struct {
	BaseMskConnect
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyMskConnect) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyMskConnectCluster ****** \n\n\n")

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
func (c *DestroyMskConnect) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyMskConnect) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListMskConnect struct {
	BaseMskConnect
}

// Execute implements the Task interface
func (c *ListMskConnect) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListMskConnectCluster ****** \n\n\n")

	if err := c.ReadMskConnectInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListMskConnect) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListMskConnect) String() string {
	return fmt.Sprintf("Echo: List MskConnect ")
}
