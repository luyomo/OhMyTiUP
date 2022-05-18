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
	//"time"
	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/ctxt"
)

type InstallTiDB struct {
	pexecutor    *ctxt.Executor
	awsWSConfigs *spec.AwsWSConfigs
}

// Execute implements the Task interface
func (c *InstallTiDB) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	nodes, err := utils.ExtractTiDBClusterNodes(clusterName, clusterType, "tidb")
	if err != nil {
		return err
	}

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 2. install docker/docker-compose/dnsutil
	if err := installPKGs(workstation, ctx, []string{"zip"}); err != nil {
		return err
	}

	// 4. Deploy docker-compose file
	type TPLParams struct {
		TiDBServers string
	}

	var tplParams TPLParams
	tplParams.TiDBServers = strings.Join((*nodes).Drainer, " ")

	if _, _, err := (*workstation).Execute(ctx, `install -d -m 0755 -o admin -g admin /opt/scripts`, true); err != nil {
		return err
	}

	fdFile, err := os.Create("/tmp/install_tidb.sh")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	err = os.Chmod("/tmp/install_tidb.sh", 0755)
	if err != nil {
		return err
	}

	fp := path.Join("templates", "scripts", "install_tidb.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(fdFile, tplParams); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/install_tidb.sh", "/opt/scripts/install_tidb.sh", false, 0)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *InstallTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *InstallTiDB) String() string {
	return fmt.Sprintf("Echo: Installing TiDB")
}
