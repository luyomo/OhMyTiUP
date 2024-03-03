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
	"text/template"
	"time"

	//	"github.com/aws/aws-sdk-go/aws"
	//	"github.com/aws/aws-sdk-go/aws/session"
	//	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type DeployPDNS struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	awsWSConfigs   *spec.AwsWSConfigs
	//	awsCloudFormationConfigs *spec.AwsCloudFormationConfigs
	//	cloudFormationType       string
	//	clusterInfo              *ClusterInfo
}

// Execute implements the Task interface
func (c *DeployPDNS) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 2. install docker/docker-compose/dnsutil
	if err := installPKGs(workstation, ctx, []string{"docker.io", "docker-compose", "dnsutils"}); err != nil {
		return err
	}

	// 3. add admin user into docker group
	if _, _, err := (*workstation).Execute(ctx, `gpasswd -a $USER docker`, true); err != nil {
		return err
	}

	nlb, err := getNLB(ctx, *c.pexecutor, clusterName, clusterType, c.subClusterType)

	// 4. Deploy docker-compose file
	type TiDBCONN struct {
		TiDBHost string
		TiDBPort int
		TiDBUser string
		TiDBPass string
		TiDBName string
	}

	var tidbConn TiDBCONN
	tidbConn.TiDBHost = *(*nlb).DNSName
	tidbConn.TiDBPort = 4000
	tidbConn.TiDBUser = "pdnsuser"
	tidbConn.TiDBPass = "pdnspass"
	tidbConn.TiDBName = "pdnsadmin"

	if _, _, err := (*workstation).Execute(ctx, `install -d -m 0755 -o admin -g admin /opt/pdns`, true); err != nil {
		return err
	}

	fdFile, err := os.Create("/tmp/pdns.yaml")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp := path.Join("templates", "docker-compose", "pdns.yaml")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(fdFile, tidbConn); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/pdns.yaml", "/opt/pdns/docker-compose.yaml", false, 0)
	if err != nil {
		return err
	}

	command := fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create user if not exists %s identified by \"%s\"'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBUser, tidbConn.TiDBPass)
	stdout, _, err := (*workstation).Execute(ctx, command, true)
	if err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName, tidbConn.TiDBUser)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns")
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns", tidbConn.TiDBUser)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		return err
	}

	// 5. Add api info into pdns.conf
	fdFile, err = os.Create("/tmp/pdns.local.conf")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp = path.Join("templates", "config", "pdns.local.conf.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	type PdnsTpl struct {
		APIKey string
	}
	var pdnsTpl PdnsTpl
	pdnsTpl.APIKey = "sfkjdhsdfsfsddffffaddfh"

	if err := tmpl.Execute(fdFile, pdnsTpl); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/pdns.local.conf", "/opt/pdns/pdns.local.conf", false, 0)
	if err != nil {
		return err
	}

	// 6. Create database for pdns
	if _, _, err := (*workstation).Execute(ctx, `docker-compose -f /opt/pdns/docker-compose.yaml up -d`, false, 300*time.Second); err != nil {
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *DeployPDNS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployPDNS) String() string {
	return fmt.Sprintf("Echo: Create CloudFormation ")
}
