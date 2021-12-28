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
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"

	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/ctxt"
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
	Threads   int
	TableSize int
}

type SysbenchTiCDC struct {
	pexecutor    *ctxt.Executor
	identityFile string
	clusterName  string
	clusterType  string
	scriptParam  ScriptParam
	clusterTable *[][]string
}

// Execute implements the Task interface
func (c *SysbenchTiCDC) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	workstation, err := getWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, "admin", c.identityFile)
	if err != nil {
		return err
	}

	var stdout []byte
	scripts := []string{"cleanup_sysbench_table.sh", "run_sysbench.sh", "analyze_sysbench.sh"}

	for _, script := range scripts {
		command := fmt.Sprintf("/opt/tidb/scripts/%s", script)
		stdout, _, err = (*workstation).Execute(ctx, command, true, time.Second*12000)
		if err != nil {
			fmt.Printf("The sysbench command is <%s>\n\n\n", command)
			fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
			return err
		}
	}

	testResult := strings.Split(strings.Replace(string(stdout), "\n", "\t", -1), "\t")

	cyan := color.New(color.FgCyan, color.Bold)

	*c.clusterTable = append(*(c.clusterTable), []string{testResult[0], cyan.Sprint(testResult[1]), testResult[2], cyan.Sprint(testResult[3])})

	return nil
}

// Rollback implements the Task interface
func (c *SysbenchTiCDC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *SysbenchTiCDC) String() string {
	return fmt.Sprintf("Echo: Creating sysbench ticdc ")
}

type PrepareSysbenchTiCDC struct {
	pexecutor    *ctxt.Executor
	identityFile string
	scriptParam  ScriptParam
}

// Execute implements the Task interface
func (c *PrepareSysbenchTiCDC) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	//	var tplParams ScriptParam
	workstation, err := getWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, "admin", c.identityFile)
	if err != nil {
		return err
	}

	//  *****   1. Fetch the TiDB host
	if c.scriptParam.TiDBUser == "" {
		tidbClusterDetail, err := getTiDBClusterInfo(workstation, ctx, clusterName, clusterType)
		if err != nil {
			return err
		}

		for _, instance := range tidbClusterDetail.Instances {
			if instance.Role == "tidb" && instance.Status == "Up" {
				fmt.Printf("The cluster detail is <%#v> \n\n\n", instance)
				c.scriptParam.TiDBHost = instance.Host
				c.scriptParam.TiDBPort = instance.Port
				c.scriptParam.TiDBDB = "cdc_test"
				c.scriptParam.TiDBUser = "root"
				break
			}
		}
	}

	// ****   2. Get aurora connection string
	dbInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "aurora")
	if err != nil {
		return err
	}
	c.scriptParam.MySQLHost = dbInstance.Endpoint.Address
	c.scriptParam.MySQLPort = dbInstance.Endpoint.Port
	c.scriptParam.MySQLDB = "cdc_test"
	c.scriptParam.MySQLUser = dbInstance.MasterUsername
	c.scriptParam.MySQLPass = "1234Abcd"

	dbInstance, err = getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "sqlserver")
	if err != nil {
		return err
	}

	c.scriptParam.MSHost = dbInstance.Endpoint.Address
	c.scriptParam.MSPort = dbInstance.Endpoint.Port
	c.scriptParam.MSDB = "cdc_test"
	c.scriptParam.MSUser = dbInstance.MasterUsername
	c.scriptParam.MSUser = "1234Abcd"

	command := fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, c.scriptParam.TiDBHost, c.scriptParam.TiDBPort, c.scriptParam.TiDBUser, c.scriptParam.TiDBDB)
	stdout, _, err := (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
		return err
	}

	arrScripts := []string{"adjust_sysbench_table.sh", "cleanup_sysbench_table.sh", "analyze_sysbench.sh", "run_sysbench.sh"}

	for _, scriptName := range arrScripts {
		err = copyTemplate(workstation, ctx, scriptName, &c.scriptParam)
		if err != nil {
			return err
		}

		stdout, _, err := (*workstation).Execute(ctx, fmt.Sprintf("chmod 755 /opt/tidb/scripts/%s", scriptName), true)
		if err != nil {
			fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
			return err
		}
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

	if c.scriptParam.TiDBPass == "" {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d cleanup", c.scriptParam.TiDBHost, c.scriptParam.TiDBUser, c.scriptParam.TiDBDB, c.scriptParam.TiDBPort, c.scriptParam.NumTables, c.scriptParam.NumTables)
	} else {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-password=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d cleanup", c.scriptParam.TiDBHost, c.scriptParam.TiDBUser, c.scriptParam.TiDBPass, c.scriptParam.TiDBDB, c.scriptParam.TiDBPort, c.scriptParam.NumTables, c.scriptParam.NumTables)
	}
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The sysbench command is <%s>\n\n\n", command)
		fmt.Printf("Errors for cleanup <%s> \n\n\n", string(stdout))
		return err
	}

	if c.scriptParam.TiDBPass == "" {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d --table-size=0 prepare", c.scriptParam.TiDBHost, c.scriptParam.TiDBUser, c.scriptParam.TiDBDB, c.scriptParam.TiDBPort, 1, c.scriptParam.NumTables)
	} else {
		command = fmt.Sprintf("sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host=%s --mysql-user=%s --mysql-password=%s --mysql-db=%s --mysql-port=%d --threads=%d --tables=%d --table-size=0 prepare", c.scriptParam.TiDBHost, c.scriptParam.TiDBUser, c.scriptParam.TiDBPass, c.scriptParam.TiDBDB, c.scriptParam.TiDBPort, 1, c.scriptParam.NumTables)
	}
	stdout, stderr, err := (*workstation).Execute(ctx, command, true, time.Second*1200)
	if err != nil {
		fmt.Printf("The sysbench command is <%s>\n\n\n", command)
		fmt.Printf("Error on the prepare <%s> \n\n\n", string(stdout))
		fmt.Printf("ERRORS: <%s> \n\n\n", string(stderr))
		return err
	}

	command = fmt.Sprintf("/opt/tidb/scripts/adjust_sysbench_table.sh")
	stdout, stderr, err = (*workstation).Execute(ctx, command, true, time.Second*12000)
	if err != nil {
		fmt.Printf("The sysbench command is <%s>\n\n\n", command)
		fmt.Printf("Adjusting sysbench errors  <%s> \n\n\n", string(stderr))
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *PrepareSysbenchTiCDC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *PrepareSysbenchTiCDC) String() string {
	return fmt.Sprintf("Echo: Preparing sysbench ticdc ")
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
