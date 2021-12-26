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
	"errors"
	"fmt"
	"strings"
	"time"

	//	"github.com/luyomo/tisample/pkg/aurora/ctxt"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"strconv"
)

type CreateDBInstance struct {
	pexecutor      *ctxt.Executor
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDBInstance) Execute(ctx context.Context) error {

	dbClusterName := fmt.Sprintf("%s-%s", c.clusterName, c.subClusterType)

	doWhenNotFound := func() error {

		command := fmt.Sprintf("aws rds create-db-instance --db-instance-identifier %s --db-cluster-identifier %s --db-parameter-group-name %s --engine aurora-mysql --engine-version 5.7.12 --db-instance-class db.r5.large --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", dbClusterName, dbClusterName, dbClusterName, c.clusterName, c.clusterType, c.subClusterType)

		err := runCreateDBInstance(*c.pexecutor, ctx, dbClusterName, command)
		if err != nil {
			return err
		}
		return nil
	}

	doWhenFound := func() error { return nil }

	if err := ExecuteDBInstance(*c.pexecutor, ctx, c.clusterName, c.clusterType, c.subClusterType, &doWhenNotFound, &doWhenFound); err != nil {
		return err
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

/******************************************************************************/

type DestroyDBInstance struct {
	pexecutor      *ctxt.Executor
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDBInstance) Execute(ctx context.Context) error {

	dbClusterName := fmt.Sprintf("%s-%s", c.clusterName, c.subClusterType)

	doWhenNotFound := func() error {
		return nil
	}

	doWhenFound := func() error {
		err := runDestroyDBInstance(*(c.pexecutor), ctx, dbClusterName)
		if err != nil {
			return err
		}
		return nil
	}

	if err := ExecuteDBInstance(*(c.pexecutor), ctx, c.clusterName, c.clusterType, c.subClusterType, &doWhenNotFound, &doWhenFound); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyDBInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDBInstance) String() string {
	return fmt.Sprintf("Echo: Generating the DB instance %s ", c.clusterName)
}

//*****************************************************************************

type CreateMS struct {
	pexecutor      *ctxt.Executor
	awsMSConfigs   *spec.AwsMSConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

func (c *CreateMS) Execute(ctx context.Context) error {

	dbClusterName := fmt.Sprintf("%s-%s", c.clusterName, c.subClusterType)

	doWhenNotFound := func() error {
		fmt.Printf(" *** *** *** Starting to create instance \n\n\n")
		command := fmt.Sprintf("aws rds create-db-instance --db-instance-identifier %s --db-instance-class %s --engine %s --master-username %s --master-user-password %s --vpc-security-group-ids %s  --db-subnet-group-name %s --db-parameter-group-name %s --engine-version %s --license-model license-included --allocated-storage %d --backup-retention-period 0 --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", dbClusterName, (*c.awsMSConfigs).InstanceType, (*c.awsMSConfigs).Engine, (*c.awsMSConfigs).DBMasterUser, (*c.awsMSConfigs).DBMasterUserPass, c.clusterInfo.privateSecurityGroupId, dbClusterName, dbClusterName, (*c.awsMSConfigs).EngineVerion, (*c.awsMSConfigs).DiskSize, c.clusterName, c.clusterType, c.subClusterType)

		err := runCreateDBInstance(*c.pexecutor, ctx, dbClusterName, command)
		if err != nil {
			return err
		}
		return nil
	}

	doWhenFound := func() error { return nil }

	if err := ExecuteDBInstance(*c.pexecutor, ctx, c.clusterName, c.clusterType, c.subClusterType, &doWhenNotFound, &doWhenFound); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateMS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateMS) String() string {
	return fmt.Sprintf("Echo: Creating MS ")
}

//******************************************************************************

func runCreateDBInstance(texecutor ctxt.Executor, ctx context.Context, dbClusterName, createCommand string) error {
	stdout, stderr, err := texecutor.Execute(ctx, createCommand, false)
	if err != nil {
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}

	var newDBInstance NewDBInstance
	if err = json.Unmarshal(stdout, &newDBInstance); err != nil {
		return err
	}

	for i := 1; i <= 1000; i++ {
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", dbClusterName)
		stdout, _, err := texecutor.Execute(ctx, command, false)
		if err != nil {
			return err
		}

		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			return err
		}

		if len(dbInstances.DBInstances) > 0 && dbInstances.DBInstances[0].DBInstanceStatus == "available" {
			return nil
		}
		time.Sleep(30 * time.Second)
	}
	return errors.New("Failed to create db instance")
}

func runDestroyDBInstance(texecutor ctxt.Executor, ctx context.Context, dbClusterName string) error {
	command := fmt.Sprintf("aws rds delete-db-instance --skip-final-snapshot --db-instance-identifier %s", dbClusterName)

	_, _, err := texecutor.Execute(ctx, command, false)
	if err != nil {
		return err
	}

	time.Sleep(120 * time.Second)
	for i := 1; i <= 1000; i++ {
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", dbClusterName)
		_, stderr, err := texecutor.Execute(ctx, command, false)
		if err != nil {
			if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", dbClusterName)) {
				return nil
			} else {
				return err
			}
		}

		time.Sleep(30 * time.Second)
	}
	return errors.New("Failed to destroy db instance")
}

func ExecuteDBInstance(texecutor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string, doWhenNotFound *func() error, doWhenFound *func() error) error {
	dbClusterName := fmt.Sprintf("%s-%s", clusterName, subClusterType)

	// Get the available zones
	command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", dbClusterName)
	stdout, stderr, err := texecutor.Execute(ctx, command, false)
	if err != nil {
		// No instance found
		if strings.Contains(string(stderr), fmt.Sprintf("DBInstance %s not found", dbClusterName)) {
			err = (*doWhenNotFound)()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}
		for _, instance := range dbInstances.DBInstances {
			existsResource := ExistsResource(clusterType, subClusterType, clusterName, instance.DBInstanceArn, texecutor, ctx)
			if existsResource == true {
				err = (*doWhenFound)()
				if err != nil {
					return err
				}

				//auroraConnInfo = instance.Endpoint
				//fmt.Printf("The db instance is+  <%#v> \n\n\n", auroraConnInfo)
				fmt.Printf("Found the instance")
				return nil
			}
		}

		return nil
	}

	return nil
}

func WaitDBInstanceUntilActive(texecutor ctxt.Executor, ctx context.Context, clusterName string) error {
	for i := 1; i <= 50; i++ {
		time.Sleep(30 * time.Second)
		command := fmt.Sprintf("aws rds describe-db-instances --db-instance-identifier '%s'", clusterName)
		stdout, stderr, err := texecutor.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error err here is <%#v> \n\n\n", err)
			fmt.Printf("The error stderr here is <%s> \n\n\n", string(stderr))
			return err
		}
		//fmt.Printf("The db cluster is <%#v>\n\n\n", string(stdout))
		var dbInstances DBInstances
		if err = json.Unmarshal(stdout, &dbInstances); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return err
		}

		if dbInstances.DBInstances[0].DBInstanceStatus == "available" {
			break
		}
	}
	return errors.New("Failed to wait until the instance becomes active")
}
