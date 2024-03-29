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

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	kmsapi "github.com/luyomo/OhMyTiUP/pkg/aws/utils/kms"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils/s3"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils/tidbcloud"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

/* **************************************************************************** */
func (b *Builder) AuroraSnapshotTaken(workstation *ws.Workstation, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &AuroraSnapshotTaken{BaseWSTask: BaseWSTask{workstation: workstation, barMessage: "Taking aurora snapshot", timer: timer}})
	return b
}

type AuroraSnapshotTaken struct {
	BaseWSTask
}

// 01. Aurora snapshot taken
// 01.01. Get Aurora connection info
// 01.02. Get binlog position
// 01.03. Make snapshot if it does not exist
func (c AuroraSnapshotTaken) Execute(ctx context.Context) error {
	c.startTimer()
	defer c.completeTimer("aurora snapshot taken")

	clusterName := ctx.Value("clusterName").(string)

	binlogPos, err := c.workstation.ReadMySQLBinPos() // Get [show master status]
	if err != nil {
		return err
	}

	snapshotARN, err := awsutils.GetSnapshot(clusterName)
	if err != nil {
		return err
	}

	if *snapshotARN == "" {
		snapshotARN, err = awsutils.RDSSnapshotTaken(clusterName, (*binlogPos)[0]["File"].(string), (*binlogPos)[0]["Position"].(float64))
		if err != nil {
			return err
		}
		// fmt.Printf("created snapshot arn : %s \n\n\n", *snapshotARN)
	}

	return nil
}

func (b *Builder) DeleteAuroraSnapshots(workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DeleteAuroraSnapshots{BaseWSTask: BaseWSTask{workstation: workstation, barMessage: "Deleting aurora snapshots ... ..."}})
	return b
}

type DeleteAuroraSnapshots struct {
	BaseWSTask
}

// 01. Aurora snapshot taken
// 01.01. Get Aurora connection info
// 01.02. Get binlog position
// 01.03. Make snapshot if it does not exist
func (c DeleteAuroraSnapshots) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	if err := awsutils.DeleteAuroraSnapshots(clusterName); err != nil {
		return err
	}

	return nil
}

/* ***************************************************************************** */

func (b *Builder) AuroraSnapshotExportS3(workstation *ws.Workstation, s3BackupFolder string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &AuroraSnapshotExportS3{
		s3BackupFolder: s3BackupFolder,
		BaseWSTask:     BaseWSTask{workstation: workstation, barMessage: "Exporting data from aurora to S3 ... ... ", timer: timer}})
	return b
}

type AuroraSnapshotExportS3 struct {
	BaseWSTask

	s3BackupFolder string
}

// 03. Data export to s3
// 03.01. Preare export role
// 03.02. Export snapshot to S3
func (c AuroraSnapshotExportS3) Execute(ctx context.Context) error {
	c.startTimer()
	defer c.completeTimer("Export snapshot to s3")

	parsedS3Dir, err := url.Parse(c.s3BackupFolder)
	if err != nil {
		return err
	}

	policy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "ExportPolicy",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject*",
                "s3:ListBucket",
                "s3:GetObject*",
                "s3:DeleteObject*",
                "s3:GetBucketLocation"
            ],
            "Resource": [
                "arn:aws:s3:::%s",
                "arn:aws:s3:::%s/*"
            ]
        }
    ]
}`, parsedS3Dir.Host, strings.Trim(parsedS3Dir.Host, "/"))

	assumeRolePolicyDocument := `{
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": {
            "Service": "export.rds.amazonaws.com"
          },
         "Action": "sts:AssumeRole"
       }
     ] 
   }`

	if err := NewBuilder().
		CreateServiceIamPolicy("s3export", policy).
		CreateServiceIamRole("s3export", assumeRolePolicyDocument).
		CreateRDSExportS3("s3export", c.s3BackupFolder, c.timer).
		Build().Execute(ctxt.New(ctx, 1)); err != nil {
		return err
	}

	return nil
}

/* ***************************************************************************** */

func (b *Builder) MakeRole4ExternalAccess(projectId string, s3url string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &MakeRole4ExternalAccess{BaseWSTask: BaseWSTask{barMessage: "Making role for external access ... ... ", timer: timer},
		tidbProjectId: projectId,
		s3url:         s3url,
	})
	return b
}

type MakeRole4ExternalAccess struct {
	BaseWSTask

	tidbProjectId string
	s3url         string
}

// 09. Create import role
// 15. TiDB Data import
func (c MakeRole4ExternalAccess) Execute(ctx context.Context) error {
	c.startTimer()
	defer c.completeTimer("Make roles for external access")

	clusterName := ctx.Value("clusterName").(string)

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = ctx.Value("clusterName").(string)
	mapArgs["clusterType"] = ctx.Value("clusterType").(string)
	mapArgs["subClusterType"] = "s3"

	kmsapi, err := kmsapi.NewKmsAPI(&mapArgs)
	if err != nil {
		return err
	}

	kmsKeys, err := kmsapi.GetKMSKey()
	if err != nil {
		return err
	}
	if kmsKeys == nil {
		return errors.New("No KMS key found")
	}

	parsedS3Url, err := url.Parse(c.s3url)
	if err != nil {
		return err
	}

	importPolicy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:GetObjectVersion"
            ],
            "Resource": "arn:aws:s3:::%s/*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation"
            ],
            "Resource": "arn:aws:s3:::%s"
        },
        {
            "Sid": "AllowKMSkey",
            "Effect": "Allow",
            "Action": [
                "kms:Decrypt"
            ],
            "Resource": "%s"
        }
    ]
}`, parsedS3Url.Host, parsedS3Url.Host, *(*kmsKeys)[0].KeyArn)

	tidbcloudApi, err := tidbcloud.NewTiDBCloudAPI(c.tidbProjectId, clusterName, nil)
	if err != nil {
		return err
	}
	accountId, externalId, err := tidbcloudApi.GetImportTaskRoleInfo()
	if err != nil {
		return err
	}
	// fmt.Printf("The account Id : %s, external id: %s \n\n\n", *accountId, *externalId)

	importAssumeRolePolicyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {
                "AWS": "%s"
            },
            "Condition": {
                "StringEquals": {
                    "sts:ExternalId": "%s"
                }
            }
        }
    ]
}`, *accountId, *externalId)

	// Need to move the KMS creation in the beginning.
	if err := NewBuilder().
		CreateServiceIamPolicy("s3import", importPolicy).
		CreateServiceIamRole("s3import", importAssumeRolePolicyDocument).
		Build().Execute(ctxt.New(ctx, 1)); err != nil {
		return err
	}
	return nil
}

/* **************************************************************************** */
func (b *Builder) DeployDM(workstation *ws.Workstation, subClusterType, version string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &DeployDM{
		BaseWSTask:     BaseWSTask{workstation: workstation, barMessage: "Deploying DM cluster ... ... ", timer: timer},
		subClusterType: subClusterType,
		version:        version,
	})
	return b
}

type DeployDM struct {
	BaseWSTask

	subClusterType string
	version        string
}

// 04. Get TiDB connection info
// 10. Install tiup
// 01. Get EC2 instances
// 11. DM deployment
// 12. Source deployment
// 13. task deployment

// 14. Diff check
func (c *DeployDM) Execute(ctx context.Context) error {
	c.startTimer()
	defer c.completeTimer("DM Deployment")

	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType
	mapArgs["subClusterType"] = c.subClusterType

	// 01. Get the instance info using AWS SDK
	ec2api, err := ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}
	dmInstances, err := ec2api.ExtractEC2Instances()
	if err != nil {
		return err
	}
	// fmt.Printf("DM instances: %#v \n\n\n\n\n\n", dmInstances)

	// 02. Take aurora connection info
	auroraConnInfo, err := c.workstation.ReadDBConnInfo(ws.DB_TYPE_AURORA)
	if err != nil {
		return err
	}
	// fmt.Printf("auroa db info: %#v \n\n\n", auroraConnInfo)

	// 03. Take binlog position / GTID
	binlogPos, err := c.workstation.ReadMySQLBinPos() // Get [show master status]
	if err != nil {
		return err
	}
	// fmt.Printf("The bionlog position: %#v \n\n\n", *binlogPos)

	// earliestBinlogPos, err := c.workstation.ReadMySQLEarliestBinPos() // Get [show master status]
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The bionlog position: %#v \n\n\n", (*earliestBinlogPos)[0])

	tidbcloudConnInfo, err := c.workstation.ReadDBConnInfo(ws.DB_TYPE_TIDBCLOUD)
	if err != nil {
		return err
	}
	// fmt.Printf("tidb cloud db info: %#v \n\n\n", tidbcloudConnInfo)

	(*tidbcloudConnInfo)["TaskName"] = clusterName
	(*tidbcloudConnInfo)["Databases"] = "test,test01"
	(*tidbcloudConnInfo)["SourceID"] = clusterName
	(*tidbcloudConnInfo)["BinlogName"] = (*binlogPos)[0]["File"].(string)
	(*tidbcloudConnInfo)["BinlogPos"] = fmt.Sprintf("%d", int((*binlogPos)[0]["Position"].(float64)))

	if err := c.workstation.InstallTiup(); err != nil {
		return err
	}

	if err := c.workstation.DeployDMCluster(clusterName, c.version, dmInstances); err != nil {
		return err
	}

	(*auroraConnInfo)["SourceName"] = clusterName
	if err := c.workstation.DeployDMSource(clusterName, auroraConnInfo); err != nil {
		return err
	}

	if err := c.workstation.DeployDMTask(clusterName, tidbcloudConnInfo); err != nil {
		return err
	}

	time.Sleep(1 * time.Minute)

	if err := c.workstation.InstallSyncDiffInspector(c.version); err != nil {
		return err
	}

	if err := c.workstation.SyncDiffInspector(clusterName, "test,test01"); err != nil {
		return err
	}

	return nil

}

// Rollback implements the Task interface
func (c *DeployDM) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployDM) String() string {
	return fmt.Sprintf("Echo: Deploying DM")
}

/* ***************************************************************************** */
func (b *Builder) DeleteS3Folder(workstation *ws.Workstation) *Builder {
	b.tasks = append(b.tasks, &DeleteS3Folder{BaseWSTask: BaseWSTask{workstation: workstation, barMessage: "Deleting S3 folder ... ... "}})
	return b
}

type DeleteS3Folder struct {
	BaseWSTask
}

// 03. Data export to s3
// 03.01. Preare export role
// 03.02. Export snapshot to S3
func (c DeleteS3Folder) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	exportTasks, err := awsutils.GetValidBackupS3(clusterName)
	if err != nil {
		return err
	}

	s3api, err := s3.NewS3API(nil)
	if err != nil {
		return err
	}

	for _, exportTask := range *exportTasks {
		if err := s3api.DeleteObject(*exportTask.S3Bucket, fmt.Sprintf("%s/%s", *exportTask.S3Prefix, *exportTask.ExportTaskIdentifier)); err != nil {
			return err
		}

		fmt.Printf("export: <%s> and <%s> \n\n\n\n\n\n", *exportTask.S3Bucket, fmt.Sprintf("%s/%s", *exportTask.S3Prefix, *exportTask.ExportTaskIdentifier))
	}
	return nil
}
