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
	"github.com/luyomo/tisample/pkg/executor"
	//	"strconv"
	"strings"
	"time"
)

type DBInstanceEndpoint struct {
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

type DBInstance struct {
	DBInstanceIdentifier string             `json:"DBInstanceIdentifier"`
	DBInstanceStatus     string             `json:"DBInstanceStatus"`
	DBInstanceArn        string             `json:"DBInstanceArn"`
	Endpoint             DBInstanceEndpoint `json:"Endpoint"`
}

type NewDBInstance struct {
	DBInstance DBInstance `json:"DBInstance"`
}

type DBInstances struct {
	DBInstances []DBInstance `json:"DBInstances"`
}

type CreateDBInstance struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

var auroraConnInfo DBInstanceEndpoint

// Execute implements the Task interface
func (c *CreateDBInstance) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", c.clusterName)
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", c.clusterName)) {
			fmt.Printf("The DB Instance has not created.\n\n\n")
		} else {
			var dbInstances DBInstances
			if err = json.Unmarshal(stdout, &dbInstances); err != nil {
				fmt.Printf("*** *** The error here is %#v \n\n", err)
				return nil
			}

			return nil
		}
	} else {
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		for _, instance := range dbInstances.DBInstances {
			existsResource := ExistsResource(c.clusterType, c.clusterName, instance.DBInstanceArn, local, ctx)
			if existsResource == true {
				auroraConnInfo = instance.Endpoint
				fmt.Printf("The db instance is+  <%#v> \n\n\n", auroraConnInfo)
				return nil
			}

		}

		return nil
	}

	command = fmt.Sprintf("aws rds create-db-instance --db-instance-identifier %s --db-cluster-identifier %s --db-parameter-group-name %s --engine aurora-mysql --engine-version 5.7.12 --db-instance-class db.r5.large --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, c.clusterName, c.clusterName, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	fmt.Printf("The db instance is <%#v>\n\n\n", string(stdout))

	var newDBInstance NewDBInstance
	if err = json.Unmarshal(stdout, &newDBInstance); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The db instance is <%#v>\n\n\n", newDBInstance)

	for i := 1; i <= 50; i++ {
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", c.clusterName)
		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
		//fmt.Printf("The db cluster is <%#v>\n\n\n", string(stdout))
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v>\n\n\n", dbInstances)
		if dbInstances.DBInstances[0].DBInstanceStatus == "available" {
			break
		}
		time.Sleep(20 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDBInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDBInstance) String() string {
	return fmt.Sprintf("Echo: Generating the DB instance %s ", c.clusterName)
}
