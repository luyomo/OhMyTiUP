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
	"encoding/json"
	"fmt"
	//	"github.com/luyomo/tisample/pkg/aurora/ctxt"
	"github.com/luyomo/tisample/pkg/aurora/executor"
	//	"strconv"
	"strings"
	//"time"
)

//type DBSubnetGroup struct {
//	DBSubnetGroupName string `json:"DBSubnetGroupName"`
//	VpcId             string `json:"VpcId"`
//	SubnetGroupStatus string `json:"SubnetGroupStatus"`
//	DBSubnetGroupArn  string `json:"DBSubnetGroupArn"`
//}

//type DBSubnetGroups struct {
//	DBSubnetGroups []DBSubnetGroup `json:"DBSubnetGroups"`
//}

type DBClusterParameterGroup struct {
	DBClusterParameterGroupName string `json:"DBClusterParameterGroupName"`
	DBParameterGroupFamily      string `json:"DBParameterGroupFamily"`
	Description                 string `json:"Description"`
	DBClusterParameterGroupArn  string `json:"DBClusterParameterGroupArn"`
}

type NewDBClusterParameterGroup struct {
	DBClusterParameterGroup DBClusterParameterGroup `json:"DBClusterParameterGroup"`
}

type CreateDBClusterParameterGroup struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateDBClusterParameterGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-cluster-parameter-groups --db-cluster-parameter-group-name 'cluster-params-%s'", "tisampletest")
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "DBClusterParameterGroup not found") {
			fmt.Printf("The DB Cluster Parameter group has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		fmt.Printf("The DB Cluster Parameter group has been created\n\n\n")
		return nil
	}

	fmt.Printf("The DB cluster oarameter <%s> \n\n\n", stdout)

	command = fmt.Sprintf("aws rds create-db-cluster-parameter-group --db-cluster-parameter-group-name cluster-params-%s --db-parameter-group-family aurora-mysql5.7 --description \"%s\"", "tisampletest", "tisampletest")
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var newDBClusterParameterGroup NewDBClusterParameterGroup
	if err = json.Unmarshal(stdout, &newDBClusterParameterGroup); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The db cluster params is <%#v>\n\n\n", newDBClusterParameterGroup)

	command = fmt.Sprintf("aws rds modify-db-cluster-parameter-group --db-cluster-parameter-group-name cluster-params-%s --parameters \"ParameterName=binlog_format,ParameterValue=row,ApplyMethod=pending-reboot\"", "tisampletest")
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The db cluster params is <%#v>\n\n\n", string(stdout))

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBClusterParameterGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBClusterParameterGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

//func groupExists(groupName string, subnetGroups []DBSubnetGroup) bool {
//	for _, theSubnetGroup := range subnetGroups {
//		if groupName == theSubnetGroup.DBSubnetGroupName {
//			return true
//		}
//	}
//	return false
//}
