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
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"

	// "go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
)

/*
componentName: alertmanager/cdc/grafana/pd/prometheus/tidb/tikv
*/
// func (b *BaseTask) getTiDBComponent(componentName string) (*[]TiDBInstanceInfo, error) {
// 	tidbClusterInfos, err := b.getTiDBClusterInfo()
// 	if err != nil {
// 		return nil, err
// 	}

// 	var tidbInstancesInfo []TiDBInstanceInfo
// 	for _, instanceInfo := range (*tidbClusterInfos).Instances {
// 		if instanceInfo.Role == componentName {
// 			tidbInstancesInfo = append(tidbInstancesInfo, instanceInfo)
// 		}
// 	}

// 	return &tidbInstancesInfo, nil

// }

// Deploy Redshift Instance
type Workstation struct {
	executor *ctxt.Executor
}

type INC_AWS_ENV_FLAG bool

const (
	INC_AWS_ENV INC_AWS_ENV_FLAG = true
	EXC_AWS_ENV INC_AWS_ENV_FLAG = false
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
}

func NewAWSWorkstation(localExe *ctxt.Executor, clusterName, clusterType, user, identityFile string, awsCliFlag INC_AWS_ENV_FLAG) (*Workstation, error) {
	var configData ConfigData
	// var user string
	// var keyFile string

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

	return &Workstation{executor: &_executor}, nil
}

// Execute implements the Task interface
func (c *Workstation) InstallPackages(packages *[]string) error {
	ctx := context.Background()

	if _, _, err := (*c.executor).Execute(ctx, "mkdir -p /opt/scripts", true); err != nil {
		return err
	}

	if _, _, err := (*c.executor).Execute(ctx, "apt-get update -y", true); err != nil {
		return err
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

func (w *Workstation) GetRedshiftDBInfo() (*RedshiftDBInfo, error) {
	var redshiftDBInfos []RedshiftDBInfo
	err := w.ParseYamlConfig("/opt/redshift.dbinfo.yaml", &redshiftDBInfos)
	if err != nil {
		return nil, err
	}
	fmt.Printf("The config is <%#v> \n\n\n", redshiftDBInfos)

	if len(redshiftDBInfos) > 1 {
		return nil, errors.New("Duplicate Redshift DB connection info")
	}
	if len(redshiftDBInfos) == 0 {
		return nil, errors.New("Redshift DB connection info not found")
	}

	return &(redshiftDBInfos[0]), nil
}

func (w *Workstation) GetTiDBDBInfo() (*DBConnectInfo, error) {
	var dbConnectInfo DBConnectInfo

	err := w.ParseYamlConfig("/opt/tidb-db-info.yml", &dbConnectInfo)
	if err != nil {
		return nil, err
	}

	return &dbConnectInfo, nil
}

func (c *Workstation) ParseYamlConfig(yamlFile string, config interface{}) error {
	localFile := fmt.Sprintf("/tmp/%s", filepath.Base(yamlFile))

	if err := (*c.executor).Transfer(context.Background(), yamlFile, localFile, true, 1024); err != nil {
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
