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
	//	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/luyomo/tisample/embed"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"text/template"
	//	"go.uber.org/zap"
	//	"math/big"
	//	"text/template"
	"time"
)

/*


  3. Get sqlserver connection string
  4. Install sysbench
  5. Initialize tables using sysbench
  6. Add columns to tidb/aurora/sqlserver
  7. Run sysbench against the TiDB
  8. Analyze the result and show it
*/

type ScriptParam struct {
	TiDBHost string
	TiDBPort int
	TiDBUser string
	TiDBPass string
	TiDBDB   string

	MySQLHost string
	MySQLPort int
	MySQLUser string
	MySQLPass string
	MySQLDB   string

	MSHost string
	MSPort int
	MSDB   string
	MSUser string
	MSPass string

	NumTables int
}

type SysbenchTiCDC struct {
	user         string
	host         string
	identityFile string
	clusterName  string
	clusterType  string
	tidbConnInfo operator.TiDBConnInfo
}

// Execute implements the Task interface
func (c *SysbenchTiCDC) Execute(ctx context.Context) error {
	fmt.Printf("Coming here for the sysbench ticdc in the task <%s> and <%s>  \n\n\n", c.clusterName, c.clusterType)
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	var tplParams ScriptParam
	//	workstation, err := getWSExecutor(local, ctx, c.clusterName, c.clusterType, "admin", c.clusterInfo.keyFile)
	workstation, err := getWSExecutor(local, ctx, c.clusterName, c.clusterType, "admin", c.identityFile)
	if err != nil {
		return err
	}

	//  *****   1. Fetch the TiDB host
	if c.tidbConnInfo.User != "" {
		tplParams.TiDBHost = c.tidbConnInfo.Host
		tplParams.TiDBPort = c.tidbConnInfo.Port
		tplParams.TiDBDB = c.tidbConnInfo.DBName
		tplParams.TiDBUser = c.tidbConnInfo.User
		tplParams.TiDBPass = c.tidbConnInfo.Pass
	} else {
		tidbClusterDetail, err := getTiDBClusterInfo(workstation, ctx, c.clusterName, c.clusterType)
		if err != nil {
			return err
		}

		for _, instance := range tidbClusterDetail.Instances {
			if instance.Role == "tidb" && instance.Status == "Up" {
				fmt.Printf("The cluster detail is <%#v> \n\n\n", instance)
				tplParams.TiDBHost = instance.Host
				tplParams.TiDBPort = instance.Port
				tplParams.TiDBDB = "cdc_test"
				tplParams.TiDBUser = "root"
				break
			}
		}
	}

	// ****   2. Get aurora connection string
	dbInstance, err := getRDBInstance(local, ctx, c.clusterName, c.clusterType, "aurora")
	if err != nil {
		return err
	}
	fmt.Printf("The aurora instance is <%#v> \n\n\n", dbInstance)
	tplParams.MySQLHost = dbInstance.Endpoint.Address
	tplParams.MySQLPort = dbInstance.Endpoint.Port
	tplParams.MySQLDB = "cdc_test"
	tplParams.MySQLUser = dbInstance.MasterUsername
	tplParams.MySQLPass = "1234Abcd"

	theEC2, err := getEC2Nodes(local, ctx, c.clusterName, c.clusterType, "sqlserver")
	if err != nil {
		return nil
	}
	fmt.Printf("The sqlserver infomatoon is <%#v> \n\n\n", theEC2)
	tplParams.MSHost = (*theEC2)[0].PrivateIpAddress
	tplParams.MSPort = 1433
	tplParams.MSDB = "cdc_test"
	tplParams.MSUser = "sa"
	tplParams.MSUser = "1234@Abcd"

	tplParams.NumTables = 30

	// 3. Get the sqlserver host

	node, err := getEC2Nodes(local, ctx, c.clusterName, c.clusterType, "sqlserver")
	if err != nil {
		return err
	}
	if len(*node) == 0 {
		return nil
	}

	deployMS2008(*workstation, ctx, (*node)[0].PrivateIpAddress)

	stdout, _, err := (*workstation).Execute(ctx, `printf \"IF (db_id('cdc_test') is null)\n  create database cdc_test;\ngo\n\" | tsql -S REPLICA -p 1433 -U sa -P 1234@Abcd`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	fmt.Printf("The parametesr are <%#v> \n\n\n", tplParams)

	err = copyTemplate(workstation, ctx, "adjust_sysbench_table.sh", &tplParams)
	if err != nil {
		return err
	}

	err = copyTemplate(workstation, ctx, "cleanup_sysbench_table.sh", &tplParams)
	if err != nil {
		return err
	}

	err = copyTemplate(workstation, ctx, "analyze_sysbench.sh", &tplParams)
	if err != nil {
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `chmod 755 /opt/tidb/scripts/adjust_sysbench_table.sh`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `chmod 755 /opt/tidb/scripts/cleanup_sysbench_table.sh`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `chmod 755 /opt/tidb/scripts/analyze_sysbench.sh`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	// sysbench install
	stdout, _, err = (*workstation).Execute(ctx, `curl -s https://packagecloud.io/install/repositories/akopytov/sysbench/script.deb.sh | sudo bash`, false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	stdout, _, err = (*workstation).Execute(ctx, `apt-get -y install sysbench`, true)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	var command string
	/*
			if tplParams.TiDBPass == "" {
				command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d cleanup", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBDB, tplParams.TiDBPort, tplParams.NumTables*2, tplParams.NumTables)
			} else {
				command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-password=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d cleanup", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBPass, tplParams.TiDBDB, tplParams.TiDBPort, tplParams.NumTables*2, tplParams.NumTables)
			}
			stdout, _, err = (*workstation).Execute(ctx, command, true)
			fmt.Printf("The sysbench command is <%s>\n\n\n", command)
			if err != nil {
				fmt.Printf("The sysbench command is <%s>\n\n\n", command)
				fmt.Printf("Errors for cleanup <%s> \n\n\n", string(stdout))
				return err
			}

			if tplParams.TiDBPass == "" {
				command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d --table-size=0 prepare", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBDB, tplParams.TiDBPort, 1, tplParams.NumTables)
			} else {
				command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-password=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d --table-size=0 prepare", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBPass, tplParams.TiDBDB, tplParams.TiDBPort, 1, tplParams.NumTables)
			}
			stdout, stderr, err := (*workstation).Execute(ctx, command, true, time.Second*120)
			if err != nil {
				fmt.Printf("The sysbench command is <%s>\n\n\n", command)
				fmt.Printf("Error on the prepare <%s> \n\n\n", string(stdout))
				fmt.Printf("Error on the prepare <%s> \n\n\n", string(stderr))
				return err
			}

		command = fmt.Sprintf("/opt/tidb/scripts/adjust_sysbench_table.sh")
		stdout, stderr, err := (*workstation).Execute(ctx, command, true, time.Second*120)
		if err != nil {
			fmt.Printf("The sysbench command is <%s>\n\n\n", command)
			fmt.Printf("Adjusting sysbench <%s> \n\n\n", string(stdout))
			fmt.Printf("Adjusting sysbench errors  <%s> \n\n\n", string(stderr))
			return err
		}
	*/

	if tplParams.TiDBPass == "" {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d --table-size=%d run", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBDB, tplParams.TiDBPort, tplParams.NumTables*150, tplParams.NumTables, 50000)
	} else {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-password=%s --mysql-db=%s --mysql-port=%d  --threads=%d --tables=%d --table-size=%d run", tplParams.TiDBHost, tplParams.TiDBUser, tplParams.TiDBPass, tplParams.TiDBDB, tplParams.TiDBPort, tplParams.NumTables*150, tplParams.NumTables, 50000)
	}
	fmt.Printf("The sysbench command is <%s>\n\n\n", command)
	stdout, _, err = (*workstation).Execute(ctx, command, true, time.Second*3600)
	if err != nil {

		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}
	fmt.Printf("The qps from sysbench is <%s> \n\n\n", string(stdout))

	command = fmt.Sprintf("/opt/tidb/scripts/analyze_sysbench.sh")
	stdout, _, err = (*workstation).Execute(ctx, command, true, time.Second*600)
	if err != nil {
		fmt.Printf("The sysbench command is <%s>\n\n\n", command)
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))

	return nil
}

// Rollback implements the Task interface
func (c *SysbenchTiCDC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *SysbenchTiCDC) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func copyTemplate(executor *ctxt.Executor, ctx context.Context, file string, tplData *ScriptParam) error {
	if stdout, stderr, err := (*executor).Execute(ctx, `mkdir -p /opt/tidb/scripts`, true); err != nil {
		fmt.Printf("The out uput is <%s> \n\n\n", string(stdout))
		fmt.Printf("The out uput error  is <%s> \n\n\n", string(stderr))
		return err
	}

	if _, _, err := (*executor).Execute(ctx, `chown -R admin:admin /opt/tidb/scripts`, true); err != nil {
		return err
	}

	fdFile, err := os.Create(fmt.Sprintf("/tmp/%s", file))
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp := path.Join("templates", "scripts", fmt.Sprintf("%s.tpl", file))
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(fdFile, *tplData); err != nil {

		return err
	}

	err = (*executor).Transfer(ctx, fmt.Sprintf("/tmp/%s", file), "/opt/tidb/scripts/", false, 0)
	if err != nil {
		fmt.Printf("The error is <%#v> \n\n\n", err)
		return err
	}

	return nil
}
