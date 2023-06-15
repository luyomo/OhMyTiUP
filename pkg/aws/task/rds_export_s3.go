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
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
)

func (b *Builder) CreateRDSExportS3(subClusterType, s3BackupFolder string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &CreateRDSExportS3{
		BaseRDSExportS3: BaseRDSExportS3{BaseTask: BaseTask{subClusterType: subClusterType, timer: timer}},
		s3BackupFolder:  s3BackupFolder,
	})
	return b
}

func (b *Builder) DestroyRDSExportS3(subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRDSExportS3{
		BaseRDSExportS3: BaseRDSExportS3{BaseTask: BaseTask{subClusterType: subClusterType}},
	})
	return b
}

/******************************************************************************/

type RDSExportS3Info struct {
	BaseResourceInfo
}

func (d *RDSExportS3Info) ToPrintTable() *[][]string {
	tableVPC := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(VPC)
		// tableVPC = append(tableVPC, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableVPC
}

func (d *RDSExportS3Info) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.ExportTask).ExportTaskIdentifier, nil
	})
}

/******************************************************************************/
type BaseRDSExportS3 struct {
	BaseTask

	// The below variables are initialized in the init() function
	client *rds.Client // Replace the example to specific service
}

func (b *BaseRDSExportS3) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = rds.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &RDSExportS3Info{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseRDSExportS3) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	exportTasks, err := awsutils.GetValidBackupS3(b.clusterName)
	if err != nil {
		return err
	}
	if exportTasks == nil {
		return nil
	}

	for _, exportTask := range *exportTasks {
		b.ResourceData.Append(exportTask)
	}
	return nil
}

/******************************************************************************/
type CreateRDSExportS3 struct {
	BaseRDSExportS3

	s3BackupFolder string
}

// Execute implements the Task interface
func (c *CreateRDSExportS3) Execute(ctx context.Context) error {
	defer c.takeTimer("Export snapshot to S3")

	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	// baseServiceIamRole := BaseServiceIamRole{BaseTask: BaseTask{subClusterType: c.subClusterType, component: c.subClusterType}}
	// baseServiceIamRole.init(ctx)
	// roleArn, err := baseServiceIamRole.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
	// if err != nil {
	// 	return err
	// }

	if clusterExistFlag == false {
		// TODO: Add resource preparation
		// tags := c.MakeEC2Tags()

		snapshotARN, err := awsutils.GetSnapshot(c.clusterName)
		if err != nil {
			return err
		}
		if *snapshotARN == "" {
			return errors.New("No snapshot found")
		}

		// 01. Get KMS
		baseKMS := BaseKMS{BaseTask: BaseTask{subClusterType: "s3"}}
		baseKMS.init(ctx)
		keyId, err := baseKMS.GetKeyId()
		if err != nil {
			return err
		}

		// 02. Get user
		baseServiceIamRole := BaseServiceIamRole{BaseTask: BaseTask{subClusterType: c.subClusterType}}
		baseServiceIamRole.init(ctx)
		roleArn, err := baseServiceIamRole.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}

		parsedS3Dir, err := url.Parse(c.s3BackupFolder)
		if err != nil {
			return err
		}

		if _, err = c.client.StartExportTask(context.TODO(), &rds.StartExportTaskInput{
			ExportTaskIdentifier: aws.String(fmt.Sprintf("%s-%s-%s", c.clusterName, c.clusterType, time.Now().Format("20060102150405"))),
			IamRoleArn:           roleArn,
			KmsKeyId:             keyId,
			S3BucketName:         aws.String(parsedS3Dir.Host),
			S3Prefix:             aws.String(strings.Trim(parsedS3Dir.Path, "/")),
			SourceArn:            snapshotARN,
		}); err != nil {
			return err
		}

		if err := c.waitUntilResouceAvailable(0, 0, 1, func() error {
			return c.readResources()
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRDSExportS3) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRDSExportS3) String() string {
	return fmt.Sprintf("Echo: Create VPC ... ...  ")
}

type DestroyRDSExportS3 struct {
	BaseRDSExportS3
}

// Execute implements the Task interface
func (c *DestroyRDSExportS3) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	// for _, rdsExportS3 := range c.ResourceData.GetData() {

	// if _, err := c.client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
	// 	VpcId: vpc.(types.Vpc).VpcId,
	// }); err != nil {
	// 	return err
	// }
	// }

	return nil
}

// Rollback implements the Task interface
func (c *DestroyRDSExportS3) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyRDSExportS3) String() string {
	return fmt.Sprintf("Echo: Destroying VPC")
}

// type ListVPC struct {
// 	BaseVPC

// 	tableVPC *[][]string
// }

// // Execute implements the Task interface
// func (c *ListVPC) Execute(ctx context.Context) error {
// 	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
// 		return err
// 	}

// 	return nil
// }

// // Rollback implements the Task interface
// func (c *ListVPC) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *ListVPC) String() string {
// 	return fmt.Sprintf("Echo: List  ")
// }
