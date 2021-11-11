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
	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/executor"
	"github.com/luyomo/tisample/pkg/aws-tidb-nodes/spec"
	"os"
	"path"
	"text/template"
)

type DeployTiDB struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
}

type TplTiupData struct {
	PD      []string
	TiDB    []string
	TiKV    []string
	TiCDC   []string
	DM      []string
	Monitor []string
}

// Execute implements the Task interface
func (c *DeployTiDB) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	fmt.Printf("Working at hte Deploy TiDB \n\n\n")
	// Filter out the instance except the terminated one.
	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=tisample-tidb\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=16\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	fmt.Printf("The instance output is <%s>\n\n\n", string(stdout))
	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			fmt.Printf("The workstation instance ... ... ... \n\n\n")
			cntInstance++
			theInstance = instance
		}
	}
	if cntInstance > 0 {
		fmt.Printf("The instance is <%s> \n\n\n", theInstance.PublicIpAddress)
	} else {
		fmt.Printf("There is no contenst here <%s> \n\n\n", string(stdout))
	}

	fmt.Printf("Reached here for the ip address \n\n\n")

	stdout, stderr, err = local.Execute(ctx, fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	if err = json.Unmarshal(stdout, &reservations); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}

	var tplData TplTiupData
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "pd" {
					tplData.PD = append(tplData.PD, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "tidb" {
					tplData.TiDB = append(tplData.TiDB, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "tikv" {
					tplData.TiKV = append(tplData.TiKV, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "ticdc" {
					tplData.TiCDC = append(tplData.TiCDC, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "dm" {
					tplData.DM = append(tplData.DM, instance.PrivateIpAddress)

				}
				if tag["Key"] == "Component" && tag["Value"] == "workstation" {
					tplData.Monitor = append(tplData.Monitor, instance.PrivateIpAddress)

				}
			}
			//			component, found := instance.Tags["Key"]
			//			fmt.Printf("The content is <%#v> \n\n\n", instance.Tags.Component)

		}
	}
	fmt.Printf("All the ip are <%#v> \n\n\n", tplData)

	//	if len(reservations.Reservations) == 0 || len(reservations.Reservations[0].Instances) == 0 {
	//	fmt.Printf("No workstation exists")
	//	return nil
	//}

	//fmt.Printf("The workstation server ip is <%#v> \n\n\n", reservations.Reservations[0].Instances[0])

	// embed/templates/config/tidb_cluster.yml.tpl

	tiupFile, err := os.Create("/tmp/tiup-test.yml")
	if err != nil {
		return err
	}
	defer tiupFile.Close()

	fp := path.Join("templates", "config", "tidb_cluster.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	//content := bytes.NewBufferString("")
	if err := tmpl.Execute(tiupFile, tplData); err != nil {
		return err
	}
	// ----- DM config file
	tiupFile, err = os.Create("/tmp/dm-test.yml")
	if err != nil {
		return err
	}
	defer tiupFile.Close()

	fp = path.Join("templates", "config", "dm_cluster.yml.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test01").Parse(string(tpl))
	if err != nil {
		return err
	}

	//content := bytes.NewBufferString("")
	if err := tmpl.Execute(tiupFile, tplData); err != nil {
		return err
	}

	// ----- DM source file
	tiupFile, err = os.Create("/tmp/dm-source.yml")
	if err != nil {
		return err
	}
	defer tiupFile.Close()

	fp = path.Join("templates", "config", "dm-source.yml.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test03").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(tiupFile, tplData); err != nil {
		return err
	}

	// ----- DM task file
	tiupFile, err = os.Create("/tmp/dm-task.yml")
	if err != nil {
		return err
	}
	defer tiupFile.Close()

	fp = path.Join("templates", "config", "dm-task.yml.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test04").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(tiupFile, tplData); err != nil {
		return err
	}

	// cdc-task.toml.tpl
	tiupFile, err = os.Create("/tmp/cdc-task.toml")
	if err != nil {
		return err
	}
	defer tiupFile.Close()

	fp = path.Join("templates", "config", "cdc-task.toml.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test05").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(tiupFile, tplData); err != nil {
		return err
	}

	//fmt.Printf("The contents is <%s> \n\n\n", string(content.Bytes()))
	// Transfer(ctx context.Context, src, dst string, download bool, limit int)

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: theInstance.PublicIpAddress, User: "admin", KeyFile: "~/.ssh/jaypingcap.pem"})
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	err = wsexecutor.Transfer(ctx, "/tmp/tiup-test.yml", "/tmp", false, 0)

	err = wsexecutor.Transfer(ctx, "/tmp/dm-test.yml", "/tmp", false, 0)

	err = wsexecutor.Transfer(ctx, "/tmp/dm-source.yml", "/tmp", false, 0)

	err = wsexecutor.Transfer(ctx, "/tmp/dm-task.yml", "/tmp", false, 0)

	err = wsexecutor.Transfer(ctx, "/tmp/cdc-task.toml", "/tmp", false, 0)

	//dm_cluster.yml.tpl
	err = wsexecutor.Transfer(ctx, "/home/pi/.ssh/jaypingcap.pem", "~/.ssh/id_rsa", false, 0)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	stdout, stderr, err = wsexecutor.Execute(ctx, `apt-get update`, true)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))

	stdout, stderr, err = wsexecutor.Execute(ctx, `curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh`, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))

	stdout, stderr, err = wsexecutor.Execute(ctx, `apt-get install -y mariadb-client-10.3`, true)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}
	fmt.Printf("The out data is <%s> \n\n\n", string(stdout))

	//stdout, stderr, err = wsexecutor.Execute(ctx, fmt.Sprintf("echo '%s' >> /tmp/tiup-%s.yml", string(content.Bytes()), c.clusterName), false)
	//if err != nil {
	//	fmt.Printf("The error here is <%#v> \n\n", err)
	//	fmt.Printf("----------\n\n")
	//	fmt.Printf("The error here is <%s> \n\n", string(stderr))
	//	return nil
	//}

	return nil

	//workstation, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	/*

		   command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=tisample-tidb},{Key=Component,Value=workstation}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.General.InstanceType, c.awsTopoConfigs.General.KeyName, clusterInfo.publicSecurityGroupId, clusterInfo.publicSubnet, c.awsTopoConfigs.General.Region, c.clusterName)
			fmt.Printf("The comamnd is <%s> \n\n\n", command)
			stdout, stderr, err = local.Execute(ctx, command, false)
			if err != nil {
				fmt.Printf("The error here is <%#v> \n\n", err)
				fmt.Printf("----------\n\n")
				fmt.Printf("The error here is <%s> \n\n", string(stderr))
				return nil
			}*/
	return nil
}

// Rollback implements the Task interface
func (c *DeployTiDB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployTiDB) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
