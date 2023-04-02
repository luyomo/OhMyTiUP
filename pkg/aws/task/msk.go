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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

func (b *Builder) CreateMSKCluster(pexecutor *ctxt.Executor, subClusterType string, awsMSKTopoConfigs *spec.AwsMSKTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsMSKTopoConfigs.CIDR
	clusterInfo.excludedAZ = awsMSKTopoConfigs.ExcludedAZ
	clusterInfo.includedAZ = awsMSKTopoConfigs.IncludedAZ
	clusterInfo.subnetsNum = awsMSKTopoConfigs.SubnetsNum

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, "nat", clusterInfo, []int{9092}).Build()).
		Step(fmt.Sprintf("%s : Creating Reshift ... ...", subClusterType), &CreateMSKCluster{
			BaseMSKCluster: BaseMSKCluster{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: "private"}, awsMSKTopoConfigs: awsMSKTopoConfigs},
			clusterInfo:    clusterInfo,
		})

	return b
}

func (b *Builder) ListMSKCluster(pexecutor *ctxt.Executor, mskInfos *MSKInfos) *Builder {
	b.tasks = append(b.tasks, &ListMSKCluster{
		BaseMSKCluster: BaseMSKCluster{BaseTask: BaseTask{pexecutor: pexecutor, ResourceData: mskInfos}},
	})
	return b
}

func (b *Builder) DestroyMSKCluster(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyMSKCluster{
		BaseMSKCluster: BaseMSKCluster{BaseTask: BaseTask{pexecutor: pexecutor}},
	})

	b.Step(fmt.Sprintf("%s : Destroying Basic resources ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())
	return b
}

type MSKInfo struct {
	ClusterName         string
	KafkaVersion        string
	State               string
	Endpoints           []string
	ClientVpcIpAddress  []string
	NumberOfBrokerNodes int32
	ClusterType         string
}

type MSKInfos struct {
	BaseResourceInfo
}

// func (d *MSKInfos) Append(cluster *types.Cluster, clusterInfo *types.ClusterInfo, listNodeInfo *[]types.NodeInfo) {
// 	var endpoints []string
// 	var clientVpcIpAddress []string
// 	for _, nodeInfo := range *listNodeInfo {
// 		endpoints = append(endpoints, nodeInfo.BrokerNodeInfo.Endpoints...)
// 		clientVpcIpAddress = append(clientVpcIpAddress, *nodeInfo.BrokerNodeInfo.ClientVpcIpAddress)
// 	}
// 	(*d).Data = append((*d).Data, MSKInfo{
// 		ClusterName:         *cluster.ClusterName,
// 		KafkaVersion:        *clusterInfo.CurrentBrokerSoftwareInfo.KafkaVersion,
// 		State:               string(cluster.State),
// 		ClusterType:         string(cluster.ClusterType),
// 		Endpoints:           endpoints,
// 		ClientVpcIpAddress:  clientVpcIpAddress,
// 		NumberOfBrokerNodes: clusterInfo.NumberOfBrokerNodes,
// 	})
// }

func (d *MSKInfos) GetFirstEndpoint() (*string, error) {

	if len((*d).Data) == 0 {
		return nil, errors.New("No MSK endpoint found")
	}
	_firstElement := ((*d).Data[0]).(MSKInfo)
	fmt.Printf("The element is <%#v> \n\n\n\n\n\n", _firstElement)

	var endpoints []string
	for _, ip := range _firstElement.ClientVpcIpAddress {
		endpoints = append(endpoints, fmt.Sprintf("%s:%s", ip, "9092"))
	}

	strEndpoints := strings.Join(endpoints, ",")
	return &strEndpoints, nil
}

func (d *MSKInfos) GetResourceArn() (*string, error) {
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource(security group) found")
	}

	return (d.Data[0]).(types.Cluster).ClusterArn, nil
}

func (d *MSKInfos) ToPrintTable() *[][]string {
	tableMSK := [][]string{{"Cluster Name", "State", "Cluster Type", "Kafka Version", "Number of Broker Nodes", "Endpoints"}}
	for _, _row := range (*d).Data {
		_entry := _row.(MSKInfo)
		tableMSK = append(tableMSK, []string{
			_entry.ClusterName,
			_entry.State,
			_entry.ClusterType,
			_entry.KafkaVersion,
			fmt.Sprintf("%d", _entry.NumberOfBrokerNodes),
			strings.Join(_entry.ClientVpcIpAddress, " , "),
		})
	}
	return &tableMSK
}

type BaseMSKCluster struct {
	BaseTask

	// pexecutor *ctxt.Executor
	client *kafka.Client // Replace the example to specific service

	// MSKInfos          *MSKInfos
	awsMSKTopoConfigs *spec.AwsMSKTopoConfigs
}

func (b *BaseMSKCluster) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = kafka.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &MSKInfos{}
	}

	if err := b.readResources(); err != nil {
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
func (b *BaseMSKCluster) ClusterExist( /*kafkaClient *kafka.Client, clusterName string,*/ checkAvailableState bool) (bool, error) {
	clusters, err := b.client.ListClustersV2(context.TODO(), &kafka.ListClustersV2Input{ClusterNameFilter: aws.String(b.clusterName)})
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters.ClusterInfoList {
		if *cluster.ClusterName == b.clusterName {
			if checkAvailableState == true {
				if cluster.State == "ACTIVE" {
					return true, nil
				} else {
					return false, nil
				}
			} else {
				return true, nil
			}

		}
	}

	return false, nil
}

func (b *BaseMSKCluster) getClusterArn( /*kafkaClient *kafka.Client, clusterName string*/ ) (*string, error) {
	clusters, err := b.client.ListClustersV2(context.TODO(), &kafka.ListClustersV2Input{ClusterNameFilter: aws.String(b.clusterName)})
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters.ClusterInfoList {
		if *cluster.ClusterName == b.clusterName {
			return cluster.ClusterArn, nil
		}
	}

	return nil, nil
}

func (b *BaseMSKCluster) ConfigurationExist( /*kafkaClient *kafka.Client, clusterName string*/ ) (bool, error) {
	configurations, err := b.client.ListConfigurations(context.TODO(), &kafka.ListConfigurationsInput{})
	if err != nil {
		return false, err
	}

	for _, configuration := range configurations.Configurations {
		if *configuration.Name == b.clusterName {
			return true, nil
		}
	}
	return false, nil
}

func (b *BaseMSKCluster) getConfigurationArn( /*kafkaClient *kafka.Client, clusterName string*/ ) (*string, error) {
	configurations, err := b.client.ListConfigurations(context.TODO(), &kafka.ListConfigurationsInput{})
	if err != nil {
		return nil, err
	}

	for _, configuration := range configurations.Configurations {
		if *configuration.Name == b.clusterName {
			return configuration.Arn, nil
		}
	}
	return nil, nil
}

func (b *BaseMSKCluster) readResources() error {

	clusters, err := b.client.ListClustersV2(context.TODO(), &kafka.ListClustersV2Input{ClusterNameFilter: aws.String(b.clusterName)})
	if err != nil {
		return err
	}
	for _, cluster := range clusters.ClusterInfoList {
		if *cluster.ClusterName == b.clusterName {
			describeCluster, err := b.client.DescribeCluster(context.TODO(), &kafka.DescribeClusterInput{ClusterArn: cluster.ClusterArn})
			if err != nil {
				return err
			}

			listNodes, err := b.client.ListNodes(context.TODO(), &kafka.ListNodesInput{ClusterArn: cluster.ClusterArn})
			if err != nil {
				return err
			}

			var endpoints []string
			var clientVpcIpAddress []string
			for _, nodeInfo := range listNodes.NodeInfoList {
				endpoints = append(endpoints, nodeInfo.BrokerNodeInfo.Endpoints...)
				clientVpcIpAddress = append(clientVpcIpAddress, *nodeInfo.BrokerNodeInfo.ClientVpcIpAddress)
			}
			mskInfo := MSKInfo{
				ClusterName:         *cluster.ClusterName,
				KafkaVersion:        string(*describeCluster.ClusterInfo.CurrentBrokerSoftwareInfo.KafkaVersion),
				State:               string(cluster.State),
				ClusterType:         string(cluster.ClusterType),
				Endpoints:           endpoints,
				ClientVpcIpAddress:  clientVpcIpAddress,
				NumberOfBrokerNodes: describeCluster.ClusterInfo.NumberOfBrokerNodes,
			}

			b.ResourceData.Append(mskInfo)
			// b.MSKInfos.Append(&cluster, describeCluster.ClusterInfo, &listNodes.NodeInfoList)
		}
	}

	return nil

}

type CreateMSKCluster struct {
	BaseMSKCluster

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateMSKCluster) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}

	// tags := []types.Tag{
	// 	{Key: aws.String("Cluster"), Value: aws.String(clusterType)},
	// 	{Key: aws.String("Type"), Value: aws.String("redshift")},
	// 	{Key: aws.String("Name"), Value: aws.String(clusterName)},
	// }

	configurationExist, err := c.ConfigurationExist()
	if err != nil {
		return err
	}

	if configurationExist == false {

		_, err := c.client.CreateConfiguration(context.TODO(), &kafka.CreateConfigurationInput{
			Name:          aws.String(c.clusterName),
			KafkaVersions: []string{"3.3.2"},
			ServerProperties: []byte(`auto.create.topics.enable=false
auto.create.topics.enable=false
default.replication.factor=3
min.insync.replicas=2
num.io.threads=8
num.network.threads=5
num.partitions=1
num.replica.fetchers=2
replica.lag.time.max.ms=30000
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600
socket.send.buffer.bytes=102400
unclean.leader.election.enable=true
zookeeper.session.timeout.ms=18000`),
		})
		if err != nil {
			return err
		}
	}

	clusterExist, err := c.ClusterExist( /*client, clusterName,*/ false)
	if err != nil {
		return err
	}

	/*
	   BadRequestException: Specify either two or three client subnets.
	*/
	// var clusterSubnets []string
	// for idx := 0; idx < 3; idx++ {
	// 	clusterSubnets = append(clusterSubnets, c.clusterInfo.privateSubnets[idx])
	// }

	clusterSubnets, err := c.GetSubnetsInfo(3)
	if err != nil {
		return err
	}
	fmt.Printf("The subnets for msk is <%#v> \n\n\n\n\n\n", clusterSubnets)

	securityGroup, err := c.GetSecurityGroup()
	if err != nil {
		return err
	}

	if clusterExist == false {
		configuratinArn, err := c.getConfigurationArn( /*client, clusterName*/ )
		if err != nil {
			return err
		}
		fmt.Printf("configuration arn: <%#v> \n\n\n\n\n\n", *configuratinArn)
		_, err = c.client.CreateClusterV2(context.TODO(), &kafka.CreateClusterV2Input{
			ClusterName: aws.String(c.clusterName),
			Provisioned: &types.ProvisionedRequest{
				KafkaVersion: aws.String("3.3.2"),
				BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
					// ClientSubnets:  clusterSubnets,
					ClientSubnets: *clusterSubnets,
					InstanceType:  aws.String(c.awsMSKTopoConfigs.InstanceType),
					// SecurityGroups: []string{c.clusterInfo.privateSecurityGroupId},
					SecurityGroups: []string{*securityGroup},
				},
				NumberOfBrokerNodes: 3,
				ClientAuthentication: &types.ClientAuthentication{
					Unauthenticated: &types.Unauthenticated{Enabled: true},
				},
				ConfigurationInfo: &types.ConfigurationInfo{
					Arn:      configuratinArn,
					Revision: 1,
				},
				EncryptionInfo: &types.EncryptionInfo{
					EncryptionInTransit: &types.EncryptionInTransit{
						ClientBroker: types.ClientBrokerPlaintext,
					},
				},
			},
		})
		if err != nil {
			return err
		}

		if err = WaitResourceUntilExpectState(60*time.Second, 60*time.Minute, func() (bool, error) {
			clusterExist, err := c.ClusterExist( /*client, clusterName,*/ true)
			return clusterExist, err
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateMSKCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateMSKCluster) String() string {
	return fmt.Sprintf("Echo: Create MSK ... ...  ")
}

type DestroyMSKCluster struct {
	BaseMSKCluster
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyMSKCluster) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := kafka.NewFromConfig(cfg)

	clusterExistFlag, err := c.ClusterExist( /*client, clusterName,*/ false)
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		clusterArn, err := c.getClusterArn( /*client, clusterName*/ )
		if err != nil {
			return err
		}

		if _, err := client.DeleteCluster(context.TODO(), &kafka.DeleteClusterInput{
			ClusterArn: clusterArn,
		}); err != nil {
			return err
		}

		if err = WaitResourceUntilExpectState(60*time.Second, 60*time.Minute, func() (bool, error) {
			clusterExist, err := c.ClusterExist( /*client, clusterName, */ false)
			return !clusterExist, err
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyMSKCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyMSKCluster) String() string {
	return fmt.Sprintf("Echo: Destroying Redshift")
}

type ListMSKCluster struct {
	BaseMSKCluster
}

// Execute implements the Task interface
func (c *ListMSKCluster) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}

	fmt.Printf("The cluste is <%#v> \n\n\n\n\n\n", c.ResourceData.GetData())

	return nil
}

// Rollback implements the Task interface
func (c *ListMSKCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListMSKCluster) String() string {
	return fmt.Sprintf("Echo: List Redshift ")
}
