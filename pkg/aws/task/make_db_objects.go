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
	"os"
	"path"

	"math/big"
	"text/template"

	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"go.uber.org/zap"
	//	"time"
)

type TplSQLServer struct {
	Name string
	Host string
	Port int
}

type CDCTaskSummary struct {
	State      string  `json:"state"`
	Tso        big.Int `json:"tso"`
	Checkpoint string  `json:"checkpoint"`
	Error      string  `json:"error"`
}

type CDCTask struct {
	Id      string         `json:"id"`
	Summary CDCTaskSummary `json:"summary"`
}

type MakeDBObjects struct {
	pexecutor      *ctxt.Executor
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

var DBNAME string

var SqlServerHost string

// Execute implements the Task interface
func (c *MakeDBObjects) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	DBNAME = "cdc_test"

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=16\"", clusterName, clusterType, "workstation")
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
	}

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			cntInstance++
			theInstance = instance
		}
	}

	command = fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
	}

	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "sqlserver" {
					SqlServerHost = instance.PrivateIpAddress
				}
			}

		}
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: theInstance.PublicIpAddress, User: "admin", KeyFile: c.clusterInfo.keyFile}, []string{})
	if err != nil {
		return nil
	}

	stdout, _, err = wsexecutor.Execute(ctx, `apt-get install -y freetds-bin`, true)
	if err != nil {
		return nil
	}

	stdout, stderr, err := wsexecutor.Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster display %s --format json `, clusterName), false)
	if err != nil {
		return err
	}

	var tidbClusterDetail TiDBClusterDetail
	if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return err
	}

	auroraInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "aurora")
	if err != nil {
		return err
	}

	hasRunTiDB := false
	for _, component := range tidbClusterDetail.Instances {
		if component.Role == "tidb" && component.Status == "Up" && hasRunTiDB == false {

			command := fmt.Sprintf(`mysql -h %s -P %d -u root -e "create database if not exists %s"`, component.Host, component.Port, DBNAME)
			stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
			if err != nil {
				return err
			}

			command = fmt.Sprintf(`mysql -h %s -P %d -u root %s -e "source /opt/tidb/sql/ontime_tidb.ddl"`, component.Host, component.Port, DBNAME)
			stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
			if err != nil {
				return err
			}

			command = fmt.Sprintf(`mysql -h %s -P %d -u master -p1234Abcd -e "create database if not exists %s"`, auroraInstance.Endpoint.Address, auroraInstance.Endpoint.Port, DBNAME)
			stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
			if err != nil {
				return err
			}

			command = fmt.Sprintf(`mysql -h %s -P %d -u master -p1234Abcd %s -e "source /opt/tidb/sql/ontime_mysql.ddl"`, auroraInstance.Endpoint.Address, auroraInstance.Endpoint.Port, DBNAME)
			stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
			if err != nil {
				return err
			}

			hasRunTiDB = true
		}

		// if component.Role == "sqlserver" && component.Status == "Up" {
		// 	fmt.Printf("Starting to run queries against sql server \n\n\n")
		// 	//command = fmt.Sprintf(`mysql -h %s -P %d -u master -p1234Abcd %s -e "source /opt/tidb/sql/ontime_mysql.ddl"`, auroraConnInfo.Address, auroraConnInfo.Port, DBNAME)
		// 	//stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
		// 	//if err != nil {
		// 	//	fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		// 	//	return err
		// 	//}
		// 	//fmt.Printf("The command is <%s> \n\n\n", command)
		// 	//fmt.Printf("The result from command <%s> \n\n\n", string(stdout))

		// 	//tsql -H 172.83.11.115 -p 1433 -U sa -P 1234@Abcd -D cdc_test
		// }
	}

	fdFile, err := os.Create(fmt.Sprintf("/tmp/%s", "freetds.conf"))
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp := path.Join("templates", "config", fmt.Sprintf("%s.tpl", "freetds.conf"))
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	var tplData TplSQLServer
	tplData.Name = "REPLICA"
	tplData.Host = SqlServerHost
	tplData.Port = 1433
	if err := tmpl.Execute(fdFile, tplData); err != nil {
		return err
	}

	err = wsexecutor.Transfer(ctx, fmt.Sprintf("/tmp/%s", "freetds.conf"), "/opt/tidb/", false, 0)
	if err != nil {
		fmt.Printf("The error is <%#v> \n\n\n", err)
	}

	command = fmt.Sprintf(`mv /opt/tidb/freetds.conf /etc/freetds/`)
	stdout, stderr, err = wsexecutor.Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		return err
	}

	command = fmt.Sprintf(`bsqldb -i /opt/tidb/freetds.conf -S %s -U sa -P 1234@Abcd -i /opt/tidb/sql/ontime_ms.ddl`, tplData.Name)
	stdout, stderr, err = wsexecutor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *MakeDBObjects) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *MakeDBObjects) String() string {
	return fmt.Sprintf("Echo: Making DB Objects ")
}
