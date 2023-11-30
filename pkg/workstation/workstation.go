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

package workstation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/utils"

	elbutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/elb"
)

// Deploy Redshift Instance
type Workstation struct {
	executor *ctxt.Executor

	mapRemoteNodes map[string]*ctxt.Executor

	ipAddr        string
	privateIPAddr string
	user          string
	identityFile  string

	tiupCmdPath string
}

type INC_AWS_ENV_FLAG bool

const (
	INC_AWS_ENV INC_AWS_ENV_FLAG = true
	EXC_AWS_ENV INC_AWS_ENV_FLAG = false
)

type IS_ROOT_USER bool

const (
	ROOT_USER     IS_ROOT_USER = true
	NON_ROOT_USER IS_ROOT_USER = false
)

// Input:
//
//	     localExe
//		clusterName
//		clusterType
//		user
//		identity
//
// awsEnv
// func NewWorkstation() (*Workstation, error) {
// 	var workstation Workstation

// }

type ConfigData struct {
	User         string `yaml:"user"`
	IdentityFile string `yaml:"identity-file"`
}

type DBConnectInfo struct {
	DBHost     string `yaml:"Host"`
	DBPort     int    `yaml:"Port"`
	DBUser     string `yaml:"User"`
	DBPassword string `yaml:"Password"`
	ProjectID  string `yaml:"ProjectID"`
}

func NewAWSWorkstation(localExe *ctxt.Executor, clusterName, clusterType, user, identityFile string, awsCliFlag INC_AWS_ENV_FLAG) (*Workstation, error) {
	var configData ConfigData

	if localExe == nil {
		return nil, errors.New("Invalid local executor")
	}

	// Lookup ssh user and private file
	// 1. Command line
	// 2. Config file
	// 3. ~/.ohmytiup/config
	if user == "" || identityFile == "" {
		configFile := fmt.Sprintf("/home/%s/.OhMyTiUP/config.yaml", utils.CurrentUser())

		if _, err := os.Stat(configFile); err == nil {
			data, err := ioutil.ReadFile(configFile)
			if err != nil {
				return nil, err
			}
			err = yaml.Unmarshal(data, &configData)
			if err != nil {
				return nil, err
			}
			user = configData.User
			identityFile = configData.IdentityFile
		}
	}

	var envs []string
	if awsCliFlag == INC_AWS_ENV {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return nil, err
		}

		envs = append(envs, fmt.Sprintf("AWS_DEFAULT_REGION=%s", cfg.Region))

		crentials, err := cfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return nil, err
		}

		envs = append(envs, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", crentials.AccessKeyID))
		envs = append(envs, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", crentials.SecretAccessKey))
		envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", crentials.SessionToken))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{clusterType}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{clusterName}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{"workstation"}})
	filters = append(filters, types.Filter{Name: aws.String("instance-state-name"), Values: []string{"running"}})

	describeInstances, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}

	if len(describeInstances.Reservations) > 1 {
		return nil, errors.New("Duplicate workstations exists.")
	}

	if len(describeInstances.Reservations) == 0 {
		return nil, errors.New("No workstation found.")
	}

	if len(describeInstances.Reservations[0].Instances) > 1 {
		return nil, errors.New("Duplicate workstations exists.")
	}

	if len(describeInstances.Reservations[0].Instances) == 0 {
		return nil, errors.New("No workstation found.")
	}

	_executor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: *describeInstances.Reservations[0].Instances[0].PublicIpAddress, User: user, KeyFile: identityFile}, envs)
	if err != nil {
		return nil, err
	}

	return &Workstation{executor: &_executor, tiupCmdPath: "$HOME/.tiup/bin", user: user, identityFile: identityFile, ipAddr: *describeInstances.Reservations[0].Instances[0].PublicIpAddress, mapRemoteNodes: make(map[string]*ctxt.Executor), privateIPAddr: *describeInstances.Reservations[0].Instances[0].PrivateIpAddress}, nil
}

func (c *Workstation) GetIPAddr() string {
	return c.privateIPAddr
}

// Execute implements the Task interface
func (c *Workstation) InstallPackages(packages *[]string) error {
	ctx := context.Background()

	if _, _, err := (*c.executor).Execute(ctx, "mkdir -p /opt/scripts", true); err != nil {
		return err
	}

	if _, _, err := (*c.executor).Execute(ctx, "mkdir -p /opt/tidb/sql", true); err != nil {
		return err
	}

	if stdout, _, err := (*c.executor).Execute(ctx, "apt-get update -y", true); err != nil {
		if strings.Contains(string(stdout), "The following signatures couldn't be verified because the public key is not available:") {
			if _, _, err := (*c.executor).Execute(ctx, "apt-get install -y aptitude", true); err != nil {
				return err
			}

			if _, _, err := (*c.executor).Execute(ctx, "aptitude install -y debian-archive-keyring", true); err != nil {
				return err
			}

			if _, _, err := (*c.executor).Execute(ctx, "apt-get update -y", true); err != nil {
				return err
			}

		} else {
			return err
		}
	}

	if packages != nil {
		for _, _package := range *packages {
			if _, _, err := (*c.executor).Execute(ctx, fmt.Sprintf("apt-get install -y %s", _package), true); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *Workstation) DeployAuroraInfo(clusterType, clusterName, password string) error {
	auroraInstanceInfos, err := awsutils.ExtractInstanceRDSInfo(clusterName, clusterType, "aurora")
	if err != nil {
		return err
	}

	ctx := context.Background()

	dbInfo := make(map[string]string)

	dbInfo["DBHost"] = (*auroraInstanceInfos)[0].EndPointAddress
	dbInfo["DBPort"] = fmt.Sprintf("%d", (*auroraInstanceInfos)[0].DBPort)
	dbInfo["DBUser"] = (*auroraInstanceInfos)[0].DBUserName
	dbInfo["DBPassword"] = password

	_, _, err = (*w.executor).Execute(ctx, "mkdir -p /opt/scripts", true)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/config/db-info.yml.tpl", "/opt/aurora-db-info.yml", "0644", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_query.sh.tpl", "/opt/scripts/run_mysql_query", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_shell_query.sh.tpl", "/opt/scripts/run_mysql_shell_query.sh", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_from_file.sh.tpl", "/opt/scripts/run_mysql_from_file", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	return nil
}

func (w *Workstation) DeployTiDBInfo(clusterName string) error {
	ctx := context.Background()

	elbapi, err := elbutils.NewELBAPI(nil)
	if err != nil {
		return err
	}
	host, err := elbapi.GetNLB(clusterName)
	if err != nil {
		return err
	}

	dbInfo := make(map[string]string)
	dbInfo["DBHost"] = *host
	dbInfo["DBPort"] = "4000"
	dbInfo["DBUser"] = "root"
	dbInfo["DBPassword"] = ""

	_, _, err = (*w.executor).Execute(ctx, "mkdir -p /opt/scripts", true)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/config/db-info.yml.tpl", "/opt/tidb-db-info.yml", "0644", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_query.sh.tpl", "/opt/scripts/run_tidb_query", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_shell_query.sh.tpl", "/opt/scripts/run_tidb_shell_query", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	err = (*w.executor).TransferTemplate(ctx, "templates/scripts/run_mysql_from_file.sh.tpl", "/opt/scripts/run_tidb_from_file", "0755", dbInfo, true, 0)
	if err != nil {
		return err
	}

	return nil
}

// Todo: Remove it after ReadDBConnInfo migration
func (w *Workstation) GetRedshiftDBInfo() (*RedshiftDBInfo, error) {
	var redshiftDBInfos []RedshiftDBInfo
	err := w.ParseYamlConfig("/opt/redshift.dbinfo.yaml", &redshiftDBInfos)
	if err != nil {
		return nil, err
	}

	if len(redshiftDBInfos) > 1 {
		return nil, errors.New("Duplicate Redshift DB connection info")
	}
	if len(redshiftDBInfos) == 0 {
		return nil, errors.New("Redshift DB connection info not found")
	}

	return &(redshiftDBInfos[0]), nil
}

// Todo: Remove it after ReadDBConnInfo migration
func (w *Workstation) GetTiDBDBInfo() (*DBConnectInfo, error) {
	var dbConnectInfo DBConnectInfo

	err := w.ParseYamlConfig("/opt/tidb-db-info.yml", &dbConnectInfo)
	if err != nil {
		return nil, err
	}

	return &dbConnectInfo, nil
}

type DB_TYPE string

const (
	DB_TYPE_AURORA    DB_TYPE = "aurora"
	DB_TYPE_TIDBCLOUD DB_TYPE = "tidbcloud"
	DB_TYPE_TIDB      DB_TYPE = "tidb"
)

func (w *Workstation) ReadDBConnInfo(dbType DB_TYPE) (*map[string]interface{}, error) {
	// var dbConnectInfo DBConnectInfo
	var dbConnectInfo map[string]interface{}

	var connFile string
	switch dbType {
	case DB_TYPE_AURORA:
		connFile = "/opt/aurora-db-info.yml"
	case DB_TYPE_TIDBCLOUD:
		connFile = "/opt/tidbcloud-info.yml"
	case DB_TYPE_TIDB:
		connFile = "/opt/tidb-info.yml"
	default:
		return nil, errors.New("Please input the db type: aurora, tidbcloud, tidb")
	}

	err := w.ParseYamlConfig(connFile, &dbConnectInfo)
	if err != nil {
		return nil, err
	}

	return &dbConnectInfo, nil
}

// Todo: Remove it after migration to ReadDBConnInfo
func (w *Workstation) ReadTiDBCloudDBInfo() (*DBConnectInfo, error) {
	var dbConnectInfo DBConnectInfo

	err := w.ParseYamlConfig("/opt/tidbcloud-info.yml", &dbConnectInfo)
	if err != nil {
		return nil, err
	}

	return &dbConnectInfo, nil
}

func (w *Workstation) ParseYamlConfig(yamlFile string, config interface{}) error {
	localFile := fmt.Sprintf("/tmp/%s", filepath.Base(yamlFile))

	if err := (*w.executor).Transfer(context.Background(), yamlFile, localFile, true, 1024); err != nil {
		return err
	}

	yfile, err := ioutil.ReadFile(localFile)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(yfile, config); err != nil {
		return err
	}

	return nil
}

func (w *Workstation) GetExecutor() (*ctxt.Executor, error) {
	if w.executor == nil {
		return nil, errors.New("Not valid workstation executor")
	}
	return w.executor, nil
}

func (w *Workstation) InstallProfiles() error {
	ctx := context.Background()
	if err := (*w.executor).Transfer(ctx, w.identityFile, "~/.ssh/id_rsa", false, 0); err != nil {
		return err
	}

	if _, _, err := (*w.executor).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false); err != nil {
		return err
	}

	return w.RunSerialCmds([]string{
		`echo "for i in ~/.profile.d/*.sh ; do
    if [ -r "\$i" ]; then
        . \$i
    fi
done" > ~/.bash_aliases`,
		"mkdir -p ~/.profile.d",
	}, false)

}

func (w *Workstation) InstallMySQLShell() error {
	if err := w.RunSerialCmds([]string{
		"wget https://downloads.mysql.com/archives/get/p/43/file/mysql-shell-8.0.33-linux-glibc2.12-x86-64bit.tar.gz -P /tmp",
		"tar xvf /tmp/mysql-shell-8.0.33-linux-glibc2.12-x86-64bit.tar.gz -C /opt --transform s/mysql-shell-8.0.33-linux-glibc2.12-x86-64bit/mysql-shell/",
		"rm -rf /tmp/mysql-shell-8.0.33-linux-glibc2.12-x86-64bit",
		"rm -rf /tmp/mysql-shell-8.0.33-linux-glibc2.12-x86-64bit.tar.gz",
		"sed -i 's/^default-character-set/#default-character-set/' /etc/mysql/mariadb.conf.d/50-client.cnf",
	}, true); err != nil {
		return err
	}

	if err := w.InstallPackages(&[]string{"jq"}); err != nil {
		return err
	}

	return w.RunSerialCmds([]string{`mkdir -p ~/.profile.d`, `echo 'export PATH=/opt/mysql-shell/bin:$PATH' > ~/.profile.d/mysql-shell.sh`}, false)

}

func (w *Workstation) RunSerialCmds(cmds []string, isRootUser bool) error {
	ctx := context.Background()

	for _, cmd := range cmds {
		if _, _, err := (*w.executor).Execute(ctx, cmd, isRootUser, 60*time.Minute); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workstation) InstallTiup() error {
	ctx := context.Background()

	_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("which %s/tiup", w.tiupCmdPath), false)
	if err != nil {
		if strings.Contains(err.Error(), "cause: exit status 1") {
			if _, _, err := (*w.executor).Execute(ctx, "curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh", false); err != nil {
				return err
			}
			_, _, err = (*w.executor).Execute(ctx, "which $HOME/.tiup/bin/tiup", false)
			if err != nil {
				return err
			}
		} else {
			return err
		}

	}

	return nil
}

func (w *Workstation) InstallEnterpriseTiup(tidbVersion string) error {
	ctx := context.Background()

	_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("which %s/tiup", w.tiupCmdPath), false)
	if err != nil {
		if strings.Contains(err.Error(), "cause: exit status 1") {
			// Need to uncomment after test.
			binTiDB := fmt.Sprintf("tidb-enterprise-server-%s-linux-amd64", tidbVersion)
			binPlugin := fmt.Sprintf("enterprise-plugin-%s-linux-amd64", tidbVersion)
			binTool := fmt.Sprintf("tidb-enterprise-toolkit-%s-linux-amd64", tidbVersion)

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("wget https://download.pingcap.org/%s.tar.gz -P /tmp", binTiDB), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("wget https://download.pingcap.org/%s.tar.gz -P /tmp", binTool), false, 600*time.Second); err != nil {
				return err
			}

			// https://download.pingcap.org/tidb-enterprise-toolkit-v7.1.0-linux-amd64.tar.gz
			// https://docs.pingcap.com/tidb/stable/upgrade-tidb-using-tiup#upgrade-tiup-offline-mirror

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("tar xvf /tmp/%s.tar.gz -C /tmp", binTiDB), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("tar xvf /tmp/%s.tar.gz -C /tmp", binTool), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("sh /tmp/%s/local_install.sh", binTiDB), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("cp -rp /tmp/%s/keys $HOME/.tiup/", binTiDB), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("$HOME/.tiup/bin/tiup mirror merge /tmp/%s", binTool), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("mkdir -p /tmp/%s", binPlugin), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("wget https://download.pingcap.org/%s.tar.gz -P /tmp", binPlugin), false, 600*time.Second); err != nil {
				return err
			}

			if _, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("tar xvf /tmp/%s.tar.gz -C /tmp/%s", binPlugin, binPlugin), false, 600*time.Second); err != nil {
				return err
			}

			_, _, err = (*w.executor).Execute(ctx, "which $HOME/.tiup/bin/tiup", false)
			if err != nil {
				return err
			}
		} else {
			return err
		}

	}

	return nil
}

func (w *Workstation) InstallSyncDiffInspector(version string) error {
	ctx := context.Background()

	installerFileName := fmt.Sprintf("tidb-community-toolkit-%s-linux-amd64", version)

	_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("which %s/sync_diff_inspector", w.tiupCmdPath), false)
	if err != nil {
		if strings.Contains(err.Error(), "cause: exit status 1") {
			if err := w.RunSerialCmds([]string{
				fmt.Sprintf("wget https://download.pingcap.org/%s.tar.gz -P /tmp", installerFileName),
				fmt.Sprintf("tar xvf /tmp/%s.tar.gz -C /tmp", installerFileName),
				// fmt.Sprintf("mv /tmp/%s/sync_diff_inspector %s/", installerFileName, w.tiupCmdPath),
				fmt.Sprintf("sudo mv /tmp/%s/sync_diff_inspector /usr/local/bin", installerFileName),
				fmt.Sprintf("rm -rf /tmp/%s", installerFileName),
				fmt.Sprintf("rm /tmp/%s.tar.gz", installerFileName),
			}, false); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (w *Workstation) InstallDumpling(version string) error {
	ctx := context.Background()

	installerFileName := fmt.Sprintf("tidb-community-toolkit-%s-linux-amd64", version)

	_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("which %s/dumpling", w.tiupCmdPath), false)
	if err != nil {
		if strings.Contains(err.Error(), "cause: exit status 1") {
			if err := w.RunSerialCmds([]string{
				fmt.Sprintf("wget https://download.pingcap.org/%s.tar.gz -P /tmp", installerFileName),
				fmt.Sprintf("tar xvf /tmp/%s.tar.gz -C /tmp", installerFileName),
				fmt.Sprintf("tar xvf /tmp/%s/dumpling-%s-linux-amd64.tar.gz -C /tmp", installerFileName, version),
				// fmt.Sprintf("mv /tmp/dumpling-%s-linux-amd64/dumpling %s/", version, w.tiupCmdPath),
				fmt.Sprintf("sudo mv /tmp/dumpling /usr/local/bin"),
				fmt.Sprintf("rm -rf /tmp/%s", installerFileName),
				fmt.Sprintf("rm -rf /tmp/dumpling-%s-linux-amd64", version),
				fmt.Sprintf("rm /tmp/%s.tar.gz", installerFileName),
			}, false); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (w *Workstation) QueryTiDB(dbName, query string) (*[]map[string]interface{}, error) {
	ctx := context.Background()

	stdout, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_shell_query %s \"%s\"", dbName, query), false, 1*time.Hour)
	if err != nil {
		return nil, err
	}

	var objmap []map[string]interface{}
	if err := json.Unmarshal(stdout, &objmap); err != nil {
		return nil, err
	}

	return &objmap, nil

}

func (w *Workstation) ExecuteTiDB(dbName, query string) error {
	ctx := context.Background()

	_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("/opt/scripts/run_tidb_query %s \"%s\"", dbName, query), false, 1*time.Hour)
	if err != nil {
		return err
	}

	return nil
}

func (w *Workstation) TransferWSFile2Remote(targetIP, sourceFile, targetFile string, isRootUser bool) error {
	ctx := context.Background()

	if isRootUser == true {
		tmpFile := fmt.Sprintf("/tmp/%d", time.Now().UnixNano())

		_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("scp -o StrictHostKeyChecking=no %s %s:%s", sourceFile, targetIP, tmpFile), false, 1*time.Hour)
		if err != nil {
			return err
		}

		if err := w.RunSerialCmdsOnRemoteNode(targetIP, []string{fmt.Sprintf("mv %s %s", tmpFile, targetFile)}, true); err != nil {
			return err
		}

	} else {
		_, _, err := (*w.executor).Execute(ctx, fmt.Sprintf("scp %s %s:%s", sourceFile, targetIP, targetFile), false, 1*time.Hour)
		if err != nil {
			return err
		}
	}

	return nil

}
