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
	// "os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect"
	"github.com/aws/aws-sdk-go-v2/service/kafkaconnect/types"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

type MSKConnectPluginInfo struct {
	ClusterName string
}

type MSKConnectPluginInfos struct {
	BaseResourceInfo
}

func (d *MSKConnectPluginInfos) Append( /*cluster *types.Cluster*/ ) {
	(*d).Data = append((*d).Data, MSKConnectPluginInfo{
		ClusterName: "MSKConnectPlugin",
	})
}

func (d *MSKConnectPluginInfos) ToPrintTable() *[][]string {
	tableMSKConnectPlugin := [][]string{{"Cluster Name"}}
	for _, _row := range (*d).Data {
		_entry := _row.(MSKConnectPluginInfo)
		tableMSKConnectPlugin = append(tableMSKConnectPlugin, []string{
			_entry.ClusterName,
		})
	}
	return &tableMSKConnectPlugin
}

type BaseMSKConnectPlugin struct {
	pexecutor                      *ctxt.Executor
	MSKConnectPluginInfos          *MSKConnectPluginInfos
	awsMSKConnectPluginTopoConfigs *spec.AwsMSKConnectPluginTopoConfigs

	// The below variables are initialized in the init() function
	client         *kafkaconnect.Client // Replace the example to specific service
	clusterName    string               // It's initialized from init() function
	clusterType    string               // It's initialized from init() function
	subClusterType string               // It's set from initializtion from caller
}

func (b *BaseMSKConnectPlugin) init(ctx context.Context) error {
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
func (b *BaseMSKConnectPlugin) ClusterExist(pluginName string) (bool, error) {
	resp, err := b.client.ListCustomPlugins(context.TODO(), &kafkaconnect.ListCustomPluginsInput{})
	if err != nil {
		return false, err
	}

	fmt.Printf("Kafka Connect Custom Plugins: <%#v>\n\n\n\n", resp)
	for _, plugin := range resp.CustomPlugins {
		if pluginName == *plugin.Name {
			return true, nil
		}
	}
	return false, nil
}

func (b *BaseMSKConnectPlugin) ReadMSKConnectPluginInfo(ctx context.Context) error {
	return nil
}

type CreateMSKConnectPlugin struct {
	BaseMSKConnectPlugin

	wsExe       *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Todo:
//
// plugin name: redshift-sink
// sink url: redshift url
// S3: directory
//
// Execute implements the Task interface
func (c *CreateMSKConnectPlugin) Execute(ctx context.Context) error {
	defer log.Infof("Completed CreateMSKConnect Plugin \n\n\n")

	c.init(ctx) // ClusterName/ClusterType and client initialization

	log.Infof("***** CreateMSKConnectPluginCluster ******")

	pluginName := fmt.Sprintf("aws-msk-%s-plugin", c.awsMSKConnectPluginTopoConfigs.Name)
	pluginFolder := fmt.Sprintf("/tmp/%s", pluginName)

	clusterExistFlag, err := c.ClusterExist(pluginName)
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		log.Infof("Started to deploy the cluster \n\n\n\n")

		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("mkdir -p %s", pluginFolder), false); err != nil {
			return err
		}

		// 1. Compile aws glue schema registry
		if err := c.compileAwsgGlueSchemaRegistry(pluginFolder); err != nil {
			return err
		}

		// 2. Download the sink
		if err := c.downloadKafkaConnectPlugin(pluginFolder); err != nil {
			return err
		}

		// 3. make the zip file and upload it to S3
		if err := c.makeZipToS3(pluginFolder); err != nil {
			return err
		}

		_, err := c.client.CreateCustomPlugin(context.TODO(), &kafkaconnect.CreateCustomPluginInput{
			Name:        aws.String(pluginName),
			ContentType: types.CustomPluginContentTypeZip,
			Location: &types.CustomPluginLocation{
				S3Location: &types.S3Location{
					BucketArn: aws.String(fmt.Sprintf("arn:aws:s3:::%s", c.awsMSKConnectPluginTopoConfigs.S3Bucket)),
					FileKey:   aws.String(fmt.Sprintf("%s/%s.zip", c.awsMSKConnectPluginTopoConfigs.S3Folder, pluginName)),
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
func (c *CreateMSKConnectPlugin) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateMSKConnectPlugin) String() string {
	return fmt.Sprintf("Echo: Create example ... ...  ")
}

func (c *CreateMSKConnectPlugin) compileAwsgGlueSchemaRegistry(pluginFolder string) error {
	defer log.Infof("Completed to compile aws glue schema registry \n\n\n")

	// 1. download source code using git
	if _, _, err := (*c.wsExe).Execute(context.Background(), "[ -d aws-glue-schema-registry ] || git clone https://github.com/awslabs/aws-glue-schema-registry.git", false); err != nil {
		return err
	}

	// 2. Compile the source code
	// 3. Make the package and dependency
	if _, _, err := (*c.wsExe).Execute(context.Background(), "export JAVA_HOME=$(readlink -f /usr/bin/javac | sed 's:/bin/javac::'); cd aws-glue-schema-registry; mvn compile; mvn package -Dmaven.test.skip=true; mvn dependency:copy-dependencies", false); err != nil {
		return err
	}

	// 4. Copy the files to target directory
	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("rm -rf %s/aws-glue-schema-registry; mkdir -p %s/aws-glue-schema-registry", pluginFolder, pluginFolder), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("cd aws-glue-schema-registry; cp avro-kafkaconnect-converter/target/schema-registry-kafkaconnect-converter-*.jar %s/aws-glue-schema-registry/", pluginFolder), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("cd aws-glue-schema-registry; cp -r avro-kafkaconnect-converter/target/dependency %s/aws-glue-schema-registry/lib", pluginFolder), false); err != nil {
		return err
	}

	return nil
}

func (c *CreateMSKConnectPlugin) downloadKafkaConnectPlugin(pluginFolder string) error {
	defer log.Infof("Completed to download the speific sink connect \n\n\n")

	connectURL := c.awsMSKConnectPluginTopoConfigs.URL
	// "https://d1i4a15mxbxib1.cloudfront.net/api/plugins/confluentinc/kafka-connect-aws-redshift/versions/1.2.2/confluentinc-kafka-connect-aws-redshift-1.2.2.zip"

	fileName := filepath.Base(connectURL)
	log.Infof("The file name is %s \n\n\n", fileName)
	extension := filepath.Ext(fileName)
	folderName := fileName[:len(fileName)-len(extension)]
	log.Infof("The file name is %s \n\n\n", folderName)

	pluginSubFolder := fmt.Sprintf("%s/%s", pluginFolder, folderName)

	// 01. Download file
	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("rm %s && wget %s", fileName, connectURL), false); err != nil {
		return err
	}

	// 02. Unzip the file and copy to target dir
	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("unzip %s", fileName), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("mkdir %s", pluginSubFolder), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("cp -r %s/lib/* %s/", folderName, pluginSubFolder), false); err != nil {
		return err
	}

	return nil
}

func (c *CreateMSKConnectPlugin) makeZipToS3(pluginFolder string) error {
	defer log.Infof("Completed to zip the sink and send it to s3 \n\n\n")

	fileName := filepath.Base(pluginFolder)

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("cd /tmp && zip -r %s.zip %s", fileName, fileName), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("aws s3 cp /tmp/%s.zip s3://%s/%s/%s.zip", fileName, c.awsMSKConnectPluginTopoConfigs.S3Bucket, c.awsMSKConnectPluginTopoConfigs.S3Folder, fileName), false); err != nil {
		return err
	}

	return nil
}

type DestroyMSKConnectPlugin struct {
	BaseMSKConnectPlugin
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyMSKConnectPlugin) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyMSKConnectPluginCluster ****** \n\n\n")

	pluginName := fmt.Sprintf("aws-msk-%s-plugin", "redshift-sink")

	clusterExistFlag, err := c.ClusterExist(pluginName)
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		// Destroy the cluster
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyMSKConnectPlugin) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyMSKConnectPlugin) String() string {
	return fmt.Sprintf("Echo: Destroying example")
}

type ListMSKConnectPlugin struct {
	BaseMSKConnectPlugin
}

// Execute implements the Task interface
func (c *ListMSKConnectPlugin) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListMSKConnectPluginCluster ****** \n\n\n")

	if err := c.ReadMSKConnectPluginInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListMSKConnectPlugin) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListMSKConnectPlugin) String() string {
	return fmt.Sprintf("Echo: List MSKConnectPlugin ")
}
