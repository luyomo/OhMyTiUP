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
	"time"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

/* **************************************************************************** */
func (b *Builder) DeployMySQL(workstation *ws.Workstation, subClusterType, version string, timer *awsutils.ExecutionTimer) *Builder {
	b.tasks = append(b.tasks, &DeployMySQL{
		BaseWSTask:     BaseWSTask{workstation: workstation, barMessage: "Deploying MySQL cluster ... ... ", timer: timer},
		subClusterType: subClusterType,
		version:        version,
	})
	return b
}

type DeployMySQL struct {
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
func (c *DeployMySQL) Execute(ctx context.Context) error {
	c.startTimer()
	defer c.completeTimer("DM Deployment")

	fmt.Printf("Starting to deploy mysql binary into each workers \n\n\n\n")

	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapArgs := make(map[string]string)
	mapArgs["Name"] = clusterName
	mapArgs["Cluster"] = clusterType
	mapArgs["Type"] = c.subClusterType

	fmt.Printf("tags: <%#v> \n\n\n", mapArgs)

	// 01. Get the instance info using AWS SDK
	ec2api, err := ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}
	mysqlInstances, err := ec2api.ExtractEC2Instances()
	if err != nil {
		return err
	}

	for _, mysqlNode := range (*mysqlInstances)["MySQLWorker"] {
		fmt.Printf("mysql nodes: <%#v> \n\n\n", mysqlNode)
		if err := c.workstation.FormatDisk(mysqlNode.(string), "/var/lib/mysql"); err != nil {
			return err
		}

		if err := c.workstation.InstallMySQLBinToWorker(mysqlNode.(string)); err != nil {
			return err
		}
	}

	if err := c.workstation.InstallSyncDiffInspector(c.version); err != nil {
		return err
	}

	if err := c.workstation.InstallDumpling(c.version); err != nil {
		return err
	}
	return nil
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

	if err := c.workstation.DeployDMCluster(clusterName, c.version, mysqlInstances); err != nil {
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
func (c *DeployMySQL) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployMySQL) String() string {
	return fmt.Sprintf("Echo: Deploying MySQL")
}
