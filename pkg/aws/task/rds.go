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
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/executor"
	"errors"
	"strings"
)

type DBInstanceEndpoint struct {
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

type DBInstance struct {
	DBInstanceIdentifier string             `json:"DBInstanceIdentifier"`
	DBInstanceStatus     string             `json:"DBInstanceStatus"`
	DBInstanceArn        string             `json:"DBInstanceArn"`
	MasterUsername       string             `json:"MasterUsername"`
	Endpoint             DBInstanceEndpoint `json:"Endpoint"`
}

type NewDBInstance struct {
	DBInstance DBInstance `json:"DBInstance"`
}

type DBInstances struct {
	DBInstances []DBInstance `json:"DBInstances"`
}

var auroraConnInfo DBInstanceEndpoint

func getRDBInstance(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string, ptrInstance *DBInstance) error {
	command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", clusterName)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", clusterName)) {
			var dbInstances DBInstances
			if err = json.Unmarshal(stdout, &dbInstances); err != nil {
				fmt.Printf("*** *** The error here is %#v \n\n", err)
				return err
			}

			return errors.New("No RDB Instance found(No matched name)")
		}
	}

	var dbInstances DBInstances
	if err = json.Unmarshal(stdout, &dbInstances); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	for _, instance := range dbInstances.DBInstances {
		existsResource := ExistsResource(clusterType, subClusterType, clusterName, instance.DBInstanceArn, executor, ctx)
		if existsResource == true {
			*ptrInstance = instance
			fmt.Printf("The db instance is+  <%#v> \n\n\n", auroraConnInfo)
			return nil
		}

	}

	return errors.New("No RDB Instance found(No mathed tags)")
}
