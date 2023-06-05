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

package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils/s3"

	"github.com/aws/smithy-go"
)

func WaitResourceUntilExpectState(_interval, _timeout time.Duration, _resourceStateCheck func() (bool, error)) error {
	timeout := time.After(_timeout)
	d := time.NewTicker(_interval)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			resourceStateAsExpectFlag, err := _resourceStateCheck()
			if err != nil {
				return err
			}
			if resourceStateAsExpectFlag == true {
				return nil
			}
		}
	}
}

type RDSInstanceInfo struct {
	PhysicalResourceId string
	EndPointAddress    string
	DBName             string
	DBPort             int64
	DBUserName         string
	DBSize             int64
	DBEngine           string
	DBEngineVersion    string
	DBInstanceClass    string
	VpcSecurityGroupId string
}

func ExtractInstanceRDSInfo(name, cluster, clusterType string) (*[]RDSInstanceInfo, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	rdsclient := rds.NewFromConfig(cfg)

	var rdsInstanceInfos []RDSInstanceInfo

	describeCNT := 100
	// Search the available RDS instance. If the instance is not available, wait until it becomes available.
	// From the logic it might have multiple RDS instance. But so far it is used for sinle instance.
	for describeCNT > 0 {
		rdsResourceInfo, err := rdsclient.DescribeDBInstances(context.TODO(), &rds.DescribeDBInstancesInput{})
		if err != nil {

			return nil, err
		}

		dbInstanceStatus := ""
		for _, dbInstance := range rdsResourceInfo.DBInstances {
			cnt := 3
			for _, tag := range dbInstance.TagList {
				if *(tag.Key) == "Name" && *(tag.Value) == name {
					cnt--
				}

				if *(tag.Key) == "Cluster" && *(tag.Value) == cluster {
					cnt--
				}

				if *(tag.Key) == "Type" && *(tag.Value) == clusterType {
					cnt--
				}
			}

			// If three tags match, go to next. Otherwise skip it
			if cnt > 0 {
				continue
			}

			// If the instance status is not available, break from the loop and wait 30 more seconds to check the status.
			dbInstanceStatus = *(dbInstance.DBInstanceStatus)
			if dbInstanceStatus != "available" {
				break
			}

			var rdsInstanceInfo RDSInstanceInfo
			if dbInstance.DBInstanceIdentifier != nil {
				rdsInstanceInfo.PhysicalResourceId = *(dbInstance.DBInstanceIdentifier)
			}
			if dbInstance.Endpoint.Address != nil {
				rdsInstanceInfo.EndPointAddress = *(dbInstance.Endpoint.Address)
			}

			if dbInstance.DBName != nil {
				rdsInstanceInfo.DBName = *(dbInstance.DBName)
			}

			rdsInstanceInfo.DBPort = int64(dbInstance.Endpoint.Port)
			if dbInstance.MasterUsername != nil {
				rdsInstanceInfo.DBUserName = *(dbInstance.MasterUsername)
			}
			rdsInstanceInfo.DBSize = int64(dbInstance.AllocatedStorage)
			if dbInstance.Engine != nil {
				rdsInstanceInfo.DBEngine = *(dbInstance.Engine)
			}
			if dbInstance.EngineVersion != nil {
				rdsInstanceInfo.DBEngineVersion = *(dbInstance.EngineVersion)
			}
			if dbInstance.DBInstanceClass != nil {
				rdsInstanceInfo.DBInstanceClass = *(dbInstance.DBInstanceClass)
			}
			if len(dbInstance.VpcSecurityGroups) > 0 {
				rdsInstanceInfo.VpcSecurityGroupId = *(dbInstance.VpcSecurityGroups[0].VpcSecurityGroupId)
			}

			rdsInstanceInfos = append(rdsInstanceInfos, rdsInstanceInfo)
		}

		// if the status is not set, no instance match.
		// If the status is set but not avaialbe, the instance might be starting. In this case, wait 30 more seconds to check the status again.
		if dbInstanceStatus != "available" && dbInstanceStatus != "" {
			time.Sleep(30 * time.Second)
			continue
		}

		break
	}

	return &rdsInstanceInfos, nil
}

func RDSSnapshotTaken(name, file string, position float64) (*string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	rdsclient := rds.NewFromConfig(cfg)

	rdsClusterInfo, err := rdsclient.DescribeDBClusters(context.TODO(), &rds.DescribeDBClustersInput{})
	if err != nil {
		return nil, err
	}
	for _, dbCluster := range rdsClusterInfo.DBClusters {
		if *dbCluster.DBClusterIdentifier == name {
			var tags []types.Tag
			tags = append(tags, types.Tag{Key: aws.String("File"), Value: aws.String(file)})
			tags = append(tags, types.Tag{Key: aws.String("Position"), Value: aws.String(fmt.Sprintf("%d", int(position)))})

			backupName := fmt.Sprintf("%s-%s-%d", name, strings.ReplaceAll(file, ".", "-"), int(position))

			snapshot, err := rdsclient.CreateDBClusterSnapshot(context.TODO(), &rds.CreateDBClusterSnapshotInput{
				DBClusterIdentifier:         dbCluster.DBClusterIdentifier,
				DBClusterSnapshotIdentifier: aws.String(backupName),
				Tags:                        tags,
			})
			if err != nil {
				return nil, err
			}

			if err = WaitResourceUntilExpectState(60*time.Second, 60*time.Minute, func() (bool, error) {
				rdsSnapshot, err := rdsclient.DescribeDBClusterSnapshots(context.TODO(), &rds.DescribeDBClusterSnapshotsInput{
					DBClusterSnapshotIdentifier: snapshot.DBClusterSnapshot.DBClusterSnapshotArn,
				})
				if err != nil {
					return false, err
				}

				if len(rdsSnapshot.DBClusterSnapshots) > 0 && *rdsSnapshot.DBClusterSnapshots[0].Status == "available" {
					return true, nil
				}

				return false, nil

			}); err != nil {
				return snapshot.DBClusterSnapshot.DBClusterSnapshotArn, err
			}

			return snapshot.DBClusterSnapshot.DBClusterSnapshotArn, nil
		}
	}

	return nil, nil
}

func DeleteAuroraSnapshots(name string) error {

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	rdsclient := rds.NewFromConfig(cfg)

	rdsSnapshot, err := rdsclient.DescribeDBClusterSnapshots(context.TODO(), &rds.DescribeDBClusterSnapshotsInput{
		DBClusterIdentifier: aws.String(name),
	})
	if err != nil {
		return err
	}

	for _, snapshot := range rdsSnapshot.DBClusterSnapshots {
		_, err := rdsclient.DeleteDBClusterSnapshot(context.TODO(), &rds.DeleteDBClusterSnapshotInput{
			DBClusterSnapshotIdentifier: snapshot.DBClusterSnapshotIdentifier,
		})
		if err != nil {
			return err
		}
	}
	return nil

}

func GetSnapshot(name string) (*string, error) {

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	rdsclient := rds.NewFromConfig(cfg)

	rdsSnapshot, err := rdsclient.DescribeDBClusterSnapshots(context.TODO(), &rds.DescribeDBClusterSnapshotsInput{
		DBClusterIdentifier: aws.String(name),
		SnapshotType:        aws.String("manual"),
	})
	if err != nil {
		return nil, err
	}

	dbClusterSnapshotArn := ""

	_file := ""
	_position := ""

	for _, snapshot := range rdsSnapshot.DBClusterSnapshots {
		localFile := ""
		localPosition := ""
		for _, tag := range snapshot.TagList {
			if *(tag.Key) == "File" {
				localFile = *(tag.Value)
			}
			if *(tag.Key) == "Position" {
				localPosition = *(tag.Value)
			}
		}
		if _file == "" && _position == "" {
			_file = localFile
			_position = localPosition
			dbClusterSnapshotArn = *snapshot.DBClusterSnapshotArn
		} else if _file <= localFile && _position <= localPosition {
			_file = localFile
			_position = localPosition
			dbClusterSnapshotArn = *snapshot.DBClusterSnapshotArn
		}
	}

	return &dbClusterSnapshotArn, nil
}

func GetValidBackupS3(snapshotName string) (*[]types.ExportTask, error) {

	// Only one snapshot is taken in the demo naming the cluster name.
	snapshotARN, err := GetSnapshot(snapshotName)
	if err != nil {
		return nil, err
	}
	if *snapshotARN == "" {
		return nil, errors.New("No snapshot found")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	rdsclient := rds.NewFromConfig(cfg)

	// Loop all the export tasks to s3. Check the meta json file to get the valid export
	resp, err := rdsclient.DescribeExportTasks(context.TODO(), &rds.DescribeExportTasksInput{SourceArn: snapshotARN})
	if err != nil {
		return nil, err
	}

	s3api, err := s3.NewS3API(nil)
	if err != nil {
		return nil, err
	}

	retExportTasks := []types.ExportTask{}
	for _, exportTask := range resp.ExportTasks {
		// b.ResourceData.Append(exportTask)
		// 01. Read the S3 contents.
		// 02. Check the content. If it's there skip it.
		if *exportTask.Status != "COMPLETE" {
			continue
		}

		file := fmt.Sprintf("%s/%s/export_info_%s.json", *exportTask.S3Prefix, *exportTask.ExportTaskIdentifier, *exportTask.ExportTaskIdentifier)
		// fmt.Printf("The file is: %s \n\n\n", file)
		err := s3api.GetObject(*exportTask.S3Bucket, file)
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				fmt.Printf("ErrorCode: %s \n\n\n", ae.ErrorCode())
				if ae.ErrorCode() == "NoSuchKey" {
					continue
				}
			}

			return nil, err
		}
		retExportTasks = append(retExportTasks, exportTask)
		// b.ResourceData.Append(exportTask)
	}
	return &retExportTasks, nil
}
