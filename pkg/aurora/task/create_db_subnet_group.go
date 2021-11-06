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

type DBSubnetGroup struct {
	DBSubnetGroupName string `json:"DBSubnetGroupName"`
	VpcId             string `json:"VpcId"`
	SubnetGroupStatus string `json:"SubnetGroupStatus"`
	DBSubnetGroupArn  string `json:"DBSubnetGroupArn"`
}

type DBSubnetGroups struct {
	DBSubnetGroups []DBSubnetGroup `json:"DBSubnetGroups"`
}

type CreateDBSubnetGroup struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateDBSubnetGroup) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	stdout, stderr, err := local.Execute(ctx, "aws rds describe-db-subnet-groups", false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var dbSubnetGroups DBSubnetGroups
	if err = json.Unmarshal(stdout, &dbSubnetGroups); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	if groupExists("tisampletest", dbSubnetGroups.DBSubnetGroups) == true {
		fmt.Printf("The db subnet group has exists \n\n\n")
		return nil
	}

	var subnets []string
	for _, subnet := range clusterInfo.subnets {
		subnets = append(subnets, "\""+subnet+"\"")
	}
	command := fmt.Sprintf("aws rds create-db-subnet-group --db-subnet-group-name %s --db-subnet-group-description \"%s\" --subnet-ids '\"'\"'[%s]'\"'\"'", "tisampletest", "tisampletest", strings.Join(subnets, ","))
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The db subnets group is <%s>\n\n\n", stdout)

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBSubnetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBSubnetGroup) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func groupExists(groupName string, subnetGroups []DBSubnetGroup) bool {
	for _, theSubnetGroup := range subnetGroups {
		if groupName == theSubnetGroup.DBSubnetGroupName {
			return true
		}
	}
	return false
}
