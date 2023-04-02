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

type ConnectorState_Process types.ConnectorState

func (p ConnectorState_Process) isState(mode ReadResourceMode) bool {
	switch mode {
	case ReadResourceModeCommon:
		return p.isOKState()
	case ReadResourceModeBeforeCreate:
		return p.isBeforeCreateState()
	case ReadResourceModeAfterCreate:
		return p.isAfterCreateState()
	case ReadResourceModeBeforeDestroy:
		return p.isBeforeDestroyState()
	case ReadResourceModeAfterDestroy:
		return p.isAfterDestroyState()
	}
	return true
}

func (p ConnectorState_Process) isBeforeCreateState() bool {
	return ListContainElement([]string{
		string(types.ConnectorStateRunning),
		string(types.ConnectorStateCreating),
		string(types.ConnectorStateUpdating),
		string(types.ConnectorStateFailed),
	}, string(p))

}

func (p ConnectorState_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string(types.ConnectorStateRunning),
		string(types.ConnectorStateFailed),
	}, string(p))

}

func (p ConnectorState_Process) isBeforeDestroyState() bool {
	return ListContainElement([]string{
		string(types.ConnectorStateRunning),
		string(types.ConnectorStateCreating),
		string(types.ConnectorStateUpdating),
		string(types.ConnectorStateFailed),
	}, string(p))

}

func (p ConnectorState_Process) isAfterDestroyState() bool {
	return ListContainElement([]string{
		string(types.ConnectorStateRunning),
		string(types.ConnectorStateCreating),
		string(types.ConnectorStateUpdating),
		string(types.ConnectorStateFailed),
	}, string(p))
}

func (p ConnectorState_Process) isOKState() bool {
	return p.isBeforeCreateState()
}

/******************************************************************************/
func (b *Builder) CreateMskConnect(wsExe *ctxt.Executor, createMskConnectInput *CreateMskConnectInput) *Builder {
	b.tasks = append(b.tasks, &CreateMskConnect{
		BaseMskConnect:        BaseMskConnect{BaseTask: BaseTask{wsExe: wsExe, subClusterType: "msk", scope: NetworkTypePrivate}},
		createMskConnectInput: createMskConnectInput,
	})
	return b
}

func (b *Builder) DestroyMskConnect(wsExe *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyMskConnect{
		BaseMskConnect: BaseMskConnect{BaseTask: BaseTask{wsExe: wsExe}},
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

// func (d *MskConnectInfos) Append( /*cluster *types.Cluster*/ ) {
// 	(*d).Data = append((*d).Data, MskConnectInfo{
// 		ClusterName: "MskConnect",
// 	})
// }

func (d *MskConnectInfos) GetResourceArn() (*string, error) {
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource(msk connect) found")
	}

	return (d.Data[0]).(types.ConnectorSummary).ConnectorArn, nil
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

func (b *BaseMskConnect) init(ctx context.Context, mode ReadResourceMode) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	log.Infof(fmt.Sprintf("config: %#v", cfg))

	b.client = kafkaconnect.NewFromConfig(cfg) // Replace the example to specific service

	if b.ResourceData == nil {
		b.ResourceData = &MskConnectInfos{}
	}

	if err := b.readResources(mode); err != nil {
		return err
	}

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

func (b *BaseMskConnect) readResources(mode ReadResourceMode) error {
	clusters, err := b.client.ListConnectors(context.TODO(), &kafkaconnect.ListConnectorsInput{})
	if err != nil {
		return err
	}
	for _, connector := range clusters.Connectors {
		if *connector.ConnectorName == b.clusterName {
			_state := ConnectorState_Process(connector.ConnectorState)
			if _state.isState(mode) == true {
				b.ResourceData.Append(connector)
			}
		}
	}
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
}

// Execute implements the Task interface
func (c *CreateMskConnect) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeAfterCreate) // ClusterName/ClusterType and client initialization

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
		// var mskConnectPluginInfos MSKConnectPluginInfos
		listMSKConnectPlugin := &ListMSKConnectPlugin{BaseMSKConnectPlugin: BaseMSKConnectPlugin{pexecutor: c.pexecutor, clusterName: c.clusterName}}
		if err := listMSKConnectPlugin.Execute(ctx); err != nil {
			return err
		}
		pluginArn, err := listMSKConnectPlugin.MSKConnectPluginInfos.GetPluginArn()
		if err != nil {
			return err
		}
		fmt.Printf("The connect plugin: <%#v>\n\n\n\n ", *pluginArn)

		// 4. Execution role
		// 5. worker configuration arn
		listWorkerConfiguration := &ListWorkerConfiguration{BaseWorkerConfiguration: BaseWorkerConfiguration{clusterName: c.clusterName}}
		if err := listWorkerConfiguration.Execute(ctx); err != nil {
			return err
		}
		workerConfigurationArn, err := listWorkerConfiguration.WorkerConfigurationInfos.GetWorkerConfigurationARN()
		if err != nil {
			return err
		}
		fmt.Printf("The worker configuration : <%#v>\n\n\n\n ", *workerConfigurationArn)

		listServiceIamRole := &ListServiceIamRole{BaseServiceIamRole: BaseServiceIamRole{BaseTask: BaseTask{pexecutor: c.pexecutor, clusterName: c.clusterName}}}
		if err := listServiceIamRole.Execute(ctx); err != nil {
			return err
		}
		roleArn, err := listServiceIamRole.ResourceData.GetResourceArn()
		if err != nil {
			return err
		}
		fmt.Printf("The role arn : <%s>\n\n\n\n ", *roleArn)

		securityGroupID, err := c.GetSecurityGroup()
		if err != nil {
			return err
		}
		fmt.Printf("The security group is : <%s> \n\n\n", *securityGroupID)

		subnets, err := c.GetSubnetsInfo(3)
		if err != nil {
			return err
		}

		fmt.Printf("The subnet is : <%s> \n\n\n", subnets)

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
						Subnets:        *subnets,
						SecurityGroups: []string{*securityGroupID},
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
						CustomPluginArn: pluginArn,
						Revision:        1,
					},
				},
			},
			ServiceExecutionRoleArn: roleArn,
			WorkerConfiguration: &types.WorkerConfiguration{
				WorkerConfigurationArn: workerConfigurationArn,
				Revision:               1,
			},
			LogDelivery: &types.LogDelivery{
				WorkerLogDelivery: &types.WorkerLogDelivery{
					S3: &types.S3LogDelivery{
						Enabled: true,
						Bucket:  aws.String("ossinsight-data"),
						Prefix:  aws.String("kafka/tidb2redshift"),
					},
				},
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
	c.init(ctx, ReadResourceModeBeforeDestroy) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyMskConnectCluster ****** \n\n\n")

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		_id, err := c.ResourceData.GetResourceArn()
		if err != nil {
			return err
		}
		_, err = c.client.DeleteConnector(context.TODO(), &kafkaconnect.DeleteConnectorInput{
			ConnectorArn: _id,
		})
		if err != nil {
			return err
		}
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
	c.init(ctx, ReadResourceModeCommon) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListMskConnectCluster ****** \n\n\n")

	// if err := c.ReadMskConnectInfo(ctx); err != nil {
	// 	return err
	// }

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
