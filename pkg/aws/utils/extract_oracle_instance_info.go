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
	// "fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type OracleInstanceInfo struct {
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

func ExtractInstanceOracleInfo(name, cluster, clusterType string) (*[]OracleInstanceInfo, error) {

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return nil, err
	}

	rdsclient := rds.NewFromConfig(cfg)

	rdsDescribeInput := &rds.DescribeDBInstancesInput{}
	var oracleInstanceInfos []OracleInstanceInfo

	describeCNT := 100
	// Search the available RDS instance. If the instance is not available, wait until it becomes available.
	// From the logic it might have multiple RDS instance. But so far it is used for sinle instance.
	for describeCNT > 0 {
		rdsResourceInfo, err := rdsclient.DescribeDBInstances(context.TODO(), rdsDescribeInput)
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

			var oracleInstanceInfo OracleInstanceInfo
			oracleInstanceInfo.PhysicalResourceId = *(dbInstance.DBInstanceIdentifier)
			oracleInstanceInfo.EndPointAddress = *(dbInstance.Endpoint.Address)
			oracleInstanceInfo.DBName = *(dbInstance.DBName)
			oracleInstanceInfo.DBPort = int64(dbInstance.Endpoint.Port)
			oracleInstanceInfo.DBUserName = *(dbInstance.MasterUsername)
			oracleInstanceInfo.DBSize = int64(dbInstance.AllocatedStorage)
			oracleInstanceInfo.DBEngine = *(dbInstance.Engine)
			oracleInstanceInfo.DBEngineVersion = *(dbInstance.EngineVersion)
			oracleInstanceInfo.DBInstanceClass = *(dbInstance.DBInstanceClass)
			oracleInstanceInfo.VpcSecurityGroupId = *(dbInstance.VpcSecurityGroups[0].VpcSecurityGroupId)

			oracleInstanceInfos = append(oracleInstanceInfos, oracleInstanceInfo)
		}

		// if the status is not set, no instance match.
		// If the status is set but not avaialbe, the instance might be starting. In this case, wait 30 more seconds to check the status again.
		if dbInstanceStatus != "available" && dbInstanceStatus != "" {
			time.Sleep(30 * time.Second)
			continue
		}

		break
	}

	return &oracleInstanceInfos, nil
}
