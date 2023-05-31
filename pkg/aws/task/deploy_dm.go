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
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"go.uber.org/zap"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	kmsapi "github.com/luyomo/OhMyTiUP/pkg/aws/utils/kms"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils/tidbcloud"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

func (b *Builder) DeployDM(pexecutor *ctxt.Executor, workstation02 *ctxt.Executor, workstation *ws.Workstation, subClusterType string, awsWSConfigs *spec.AwsWSConfigs, tidbCloudConfigs *spec.TiDBCloudConfigs) *Builder {
	b.tasks = append(b.tasks, &DeployDM{
		pexecutor:        pexecutor,
		workstation02:    workstation02,
		workstation:      workstation,
		awsWSConfigs:     awsWSConfigs,
		tidbCloudConfigs: tidbCloudConfigs,
		subClusterType:   subClusterType,
	})
	return b
}

/* **************************************************************************** */

type DeployDM struct {
	pexecutor        *ctxt.Executor
	workstation02    *ctxt.Executor
	workstation      *ws.Workstation
	awsWSConfigs     *spec.AwsWSConfigs
	tidbCloudConfigs *spec.TiDBCloudConfigs
	subClusterType   string
}

type TplTiupDMData struct {
	DMMaster     []string
	DMWorker     []string
	Monitor      []string
	Grafana      []string
	AlertManager []string
	TaskMetaData struct {
		Host       string
		Port       int32
		User       string
		Password   string
		TaskName   string
		Databases  []string
		SourceID   string
		BinlogName string
		BinlogPos  int
	}
}

func (t TplTiupDMData) String() string {
	return fmt.Sprintf("DM Master: %s  |  DMWorker:%s  | Monitor:%s | AlertManager: %s | Grafana: %s", strings.Join(t.DMMaster, ","), strings.Join(t.DMWorker, ","), strings.Join(t.Monitor, ","), strings.Join(t.Grafana, ","), strings.Join(t.AlertManager, ","))
}

// 04. Task aurora snapshot
// 05. Take aurora database snapshot to S3
// 06. Import snapshot data from S3 to TiDBCloud
// 07. Take the TiDB Cloud connection info
// 08. Create source
// 09. Create task
// Execute implements the Task interface
func (c *DeployDM) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType
	mapArgs["subClusterType"] = c.subClusterType

	// 01. Get the instance info using AWS SDK
	ec2api, err := ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}
	dmInstances, err := ec2api.ExtractEC2Instances(clusterName, clusterType, "")
	if err != nil {
		return err
	}
	fmt.Printf("DM instances: %#v \n\n\n\n\n\n", dmInstances)

	// 02. Take aurora connection info
	auroraConnInfo, err := c.workstation.ReadDBConnInfo(ws.DB_TYPE_AURORA)
	if err != nil {
		return err
	}
	fmt.Printf("auroa db info: %#v \n\n\n", auroraConnInfo)

	// 03. Take binlog position / GTID
	binlogPos, err := c.workstation.ReadMySQLBinPos() // Get [show master status]
	if err != nil {
		return err
	}
	fmt.Printf("The bionlog position: %#v \n\n\n", *binlogPos)

	earliestBinlogPos, err := c.workstation.ReadMySQLEarliestBinPos() // Get [show master status]
	if err != nil {
		return err
	}
	fmt.Printf("The bionlog position: %#v \n\n\n", (*earliestBinlogPos)[0])

	// snapshotARN, err := awsutils.GetSnapshot(clusterName, (*binlogPos)[0]["File"].(string), (*binlogPos)[0]["Position"].(float64))
	snapshotARN, err := awsutils.GetSnapshot(clusterName)
	if err != nil {
		return err
	}
	fmt.Printf("---------- The snapshot arn : %s \n\n\n", *snapshotARN)

	if *snapshotARN == "" {
		snapshotARN, err = awsutils.RDSSnapshotTaken(clusterName, (*binlogPos)[0]["File"].(string), (*binlogPos)[0]["Position"].(float64))
		if err != nil {
			return err
		}
		fmt.Printf("created snapshot arn : %s \n\n\n", *snapshotARN)
	}
	fmt.Printf("fetched snapshot arn : %s \n\n\n", *snapshotARN)

	policy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "ExportPolicy",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject*",
                "s3:ListBucket",
                "s3:GetObject*",
                "s3:DeleteObject*",
                "s3:GetBucketLocation"
            ],
            "Resource": [
                "arn:aws:s3:::%s",
                "arn:aws:s3:::%s/*"
            ]
        }
    ]
}`, "jay-data", "jay-data")

	assumeRolePolicyDocument := `{
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": {
            "Service": "export.rds.amazonaws.com"
          },
         "Action": "sts:AssumeRole"
       }
     ] 
   }`

	mapArgs["subClusterType"] = "s3"
	kmsapi, err := kmsapi.NewKmsAPI(&mapArgs)
	if err != nil {
		return err
	}

	kmsKeys, err := kmsapi.GetKMSKey()
	if err != nil {
		return err
	}
	if kmsKeys == nil {
		return errors.New("No KMS key found")
	}

	importPolicy := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:GetObjectVersion"
            ],
            "Resource": "arn:aws:s3:::%s/*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation"
            ],
            "Resource": "arn:aws:s3:::%s"
        },
        {
            "Sid": "AllowKMSkey",
            "Effect": "Allow",
            "Action": [
                "kms:Decrypt"
            ],
            "Resource": "%s"
        }
    ]
}`, "jay-data", "jay-data", *(*kmsKeys)[0].KeyArn)

	tidbcloudApi, err := tidbcloud.NewTiDBCloudAPI(c.tidbCloudConfigs.TiDBCloudProjectID, clusterName, nil)
	if err != nil {
		return err
	}
	accountId, externalId, err := tidbcloudApi.GetImportTaskRoleInfo()
	if err != nil {
		return err
	}
	fmt.Printf("The account Id : %s, external id: %s \n\n\n", *accountId, *externalId)

	importAssumeRolePolicyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sts:AssumeRole",
            "Principal": {
                "AWS": "%s"
            },
            "Condition": {
                "StringEquals": {
                    "sts:ExternalId": "%s"
                }
            }
        }
    ]
}`, *accountId, *externalId)

	if err := NewBuilder().
		CreateServiceIamPolicy("s3export", policy).
		CreateServiceIamRole("s3export", assumeRolePolicyDocument).
		CreateKMS("s3").
		CreateRDSExportS3("s3export").
		CreateServiceIamPolicy("s3import", importPolicy).
		CreateServiceIamRole("s3import", importAssumeRolePolicyDocument).
		Build().Execute(ctxt.New(ctx, 1)); err != nil {
		return err
	}

	// if err := tasks.Execute(ctxt.New(ctx, 10)); err != nil {
	// 	return err
	// }

	// fmt.Printf("The tasks are <%#v> \n\n\n", tasks)

	// Cloud formation
	// -- Create S3 bucket
	// -- Create IAM policy
	// -- Create IAM role

	return nil
	return errors.New("stop here")

	// Get the earliest bin position
	// Get the current bin position
	// Get the bin position from snapshot

	// 2. Get all the nodes from tag definition
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	var tplDMData TplTiupDMData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {

				if tag["Key"] == "Component" && tag["Value"] == "dm-master" {
					tplDMData.DMMaster = append(tplDMData.DMMaster, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "dm-worker" {
					tplDMData.DMWorker = append(tplDMData.DMWorker, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.Grafana = append(tplDMData.Grafana, instance.PrivateIpAddress)
				}

				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.Monitor = append(tplDMData.Monitor, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplDMData.AlertManager = append(tplDMData.AlertManager, instance.PrivateIpAddress)

				}
			}
		}
	}

	// TiDB Cloud connection info
	tplDMData.TaskMetaData.Host = c.tidbCloudConfigs.Host
	tplDMData.TaskMetaData.Port = c.tidbCloudConfigs.Port
	tplDMData.TaskMetaData.User = c.tidbCloudConfigs.User
	tplDMData.TaskMetaData.Password = c.tidbCloudConfigs.Password
	tplDMData.TaskMetaData.Databases = c.tidbCloudConfigs.Databases

	// Master data
	tplDMData.TaskMetaData.TaskName = clusterName
	tplDMData.TaskMetaData.SourceID = clusterName

	// Meta data from aurora
	tplDMData.TaskMetaData.BinlogName = "mysql-bin-changelog.000003"
	tplDMData.TaskMetaData.BinlogPos = 154

	zap.L().Debug("AWS WS Config:", zap.String("Monitoring", c.awsWSConfigs.EnableMonitoring))
	if c.awsWSConfigs.EnableMonitoring == "enabled" {
		workstation, err := GetWorkstation(*c.pexecutor, ctx)
		if err != nil {
			return err
		}
		tplDMData.Monitor = append(tplDMData.Monitor, workstation.PrivateIpAddress)
	}
	zap.L().Debug("Deploy server info:", zap.String("deploy servers", tplDMData.String()))

	// 3. Make all the necessary folders
	if _, _, err := (*c.workstation02).Execute(ctx, `mkdir -p /opt/tidb/sql`, true); err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, fmt.Sprintf(`chown -R %s:%s /opt/tidb`, c.awsWSConfigs.UserName, c.awsWSConfigs.UserName), true); err != nil {
		return err
	}

	var dbInfo DBInfo
	dbInfo.DBHost = c.tidbCloudConfigs.Host
	dbInfo.DBPort = int64(c.tidbCloudConfigs.Port)
	dbInfo.DBUser = c.tidbCloudConfigs.User
	dbInfo.DBPassword = c.tidbCloudConfigs.Password

	if err = TransferToWorkstation(c.workstation02, "templates/config/db-info.yml.tpl", "/opt/tidbcloud-info.yml", "0644", dbInfo); err != nil {
		return err
	}

	// ************************** Cluster setup **************************
	// The below logic is better to move to workstation

	if err := c.workstation.DeployDMCluster(); err != nil {
		return err
	}

	// 4. Deploy all tidb templates
	configFiles := []string{"dm-task.yml", "dm-cluster.yml"}
	for _, configFile := range configFiles {
		fdFile, err := os.Create(fmt.Sprintf("/tmp/%s", configFile))
		if err != nil {
			return err
		}
		defer fdFile.Close()

		fp := path.Join("templates", "config", fmt.Sprintf("%s.tpl", configFile))
		tpl, err := embed.ReadTemplate(fp)
		if err != nil {
			return err
		}

		tmpl, err := template.New("test").Parse(string(tpl))
		if err != nil {
			return err
		}

		if err := tmpl.Execute(fdFile, tplDMData); err != nil {
			return err
		}

		err = (*c.workstation02).Transfer(ctx, fmt.Sprintf("/tmp/%s", configFile), "/opt/tidb/", false, 0)
		if err != nil {
			return err
		}
	}

	// // 6. Send the access key to workstation
	// err = (*c.workstation).Transfer(ctx, c.awsWSConfigs.KeyFile, "~/.ssh/id_rsa", false, 0)
	// if err != nil {
	// 	return err
	// }

	// stdout, _, err = (*c.workstation).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false)
	// if err != nil {
	// 	return err
	// }

	// 7. Add limit configuration, otherwise the configuration will impact the performance test with heavy load.
	/*
	 * hard nofile 65535
	 * soft nofile 65535
	 */
	// err = (*c.workstation).Transfer(ctx, "embed/templates/config/limits.conf", "/tmp", false, 0)
	// if err != nil {
	// 	return err
	// }

	// _, _, err = (*c.workstation).Execute(ctx, `mv /tmp/limits.conf /etc/security/limits.conf`, true)
	// if err != nil {
	// 	return err

	// }

	stdout, _, err = (*c.workstation02).Execute(ctx, `apt-get update`, true)
	if err != nil {
		return err
	}

	stdout, _, err = (*c.workstation02).Execute(ctx, `curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh`, false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	if err := installPKGs(c.workstation02, ctx, []string{"mariadb-client-10.3"}); err != nil {
		return err
	}

	dmClusterInfo, err := getDMClusterInfo(c.workstation02, ctx, clusterName)
	if err != nil {
		return err
	}

	if dmClusterInfo == nil {
		stdout, _, err = (*c.workstation02).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm deploy %s %s %s -y", clusterName, "v6.1.0", "/opt/tidb/dm-cluster.yml"), false)
		if err != nil {
			fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
			return err
		}
	}

	stdout, _, err = (*c.workstation02).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dm start %s", clusterName), false)
	if err != nil {
		fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
		return err
	}

	if err = (*c.workstation02).Transfer(ctx, "/opt/aurora-db-info.yml", "/tmp/aurora-db-info.yml", true, 1024); err != nil {
		return err
	}

	type SourceData struct {
		MySQLHost     string `yaml:"Host"`
		MySQLPort     int    `yaml:"Port"`
		MySQLUser     string `yaml:"User"`
		MySQLPassword string `yaml:"Password"`

		SourceName string
	}

	sourceData := SourceData{}

	yfile, err := ioutil.ReadFile("/tmp/aurora-db-info.yml")
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(yfile, &sourceData); err != nil {
		return err
	}
	sourceData.SourceName = clusterName

	err = (*c.workstation02).TransferTemplate(ctx, "templates/config/dm-source.yml.tpl", "/tmp/dm-source.yml", "0644", sourceData, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, "mv /tmp/dm-source.yml /opt/tidb/", true); err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, "wget https://download.pingcap.org/tidb-community-toolkit-v6.1.0-linux-amd64.tar.gz", true, time.Second*600); err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, "tar xvf tidb-community-toolkit-v6.1.0-linux-amd64.tar.gz", true); err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, "mv tidb-community-toolkit-v6.1.0-linux-amd64/sync_diff_inspector /home/admin/.tiup/bin/", true); err != nil {
		return err
	}

	if _, _, err := (*c.workstation02).Execute(ctx, "rm -rf tidb-community-toolkit-v6.1.0-linux-amd64; rm tidb-community-toolkit-v6.1.0-linux-amd64.tar.gz", true); err != nil {
		return err
	}

	type SyncDiffCheck struct {
		CheckThreadCount int
		MasterNode       string
		DMTaskName       string
		DMOutputDir      string
		Databases        []string
	}

	syncDiffCheck := SyncDiffCheck{
		CheckThreadCount: 4,
		MasterNode:       fmt.Sprintf("%s:%d", tplDMData.DMMaster[0], 8261),
		DMTaskName:       clusterName,
		DMOutputDir:      "/tmp/output",
		Databases:        c.tidbCloudConfigs.Databases,
	}

	if err = TransferToWorkstation(c.workstation02, "templates/config/dm-sync-diff-check.toml.tpl", "/opt/dm-sync-diff-check.toml", "0644", syncDiffCheck); err != nil {
		return err
	}

	// stdout, _, err = (*workstation).Execute(ctx, fmt.Sprintf("/home/admin/.tiup/bin/tiup dmctl --master-addr %s:8261 operate-source create /opt/tidb/dm-source.yml", tplDMData.DMMaster[0]), false)
	// if err != nil {
	// 	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))
	// 	return err
	// }

	return nil
}

// Rollback implements the Task interface
func (c *DeployDM) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployDM) String() string {
	return fmt.Sprintf("Echo: Deploying DM")
}
