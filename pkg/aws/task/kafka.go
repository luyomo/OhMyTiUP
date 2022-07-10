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
	"time"
	// "os"
	// "path"
	// "strings"
	// "text/template"

	// "github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

type DeployKafka struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

type KafkaNodes struct {
	All            []string
	Zookeeper      []string
	Broker         []string
	SchemaRegistry []string
}

// Execute implements the Task interface
func (c *DeployKafka) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	// 2. Send the access key to workstation
	err = (*workstation).Transfer(ctx, c.clusterInfo.keyFile, "~/.ssh/id_rsa", false, 0)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, `apt-get update`, true)
	if err != nil {
		return err
	}

	// 3. Get all the nodes from tag definition
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

	var kafkaNodes KafkaNodes
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag["Key"] == "Component" && tag["Value"] == "zookeeper" {
					kafkaNodes.Zookeeper = append(kafkaNodes.Zookeeper, instance.PrivateIpAddress)
					kafkaNodes.All = append(kafkaNodes.All, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "broker" {
					kafkaNodes.Broker = append(kafkaNodes.Broker, instance.PrivateIpAddress)
					kafkaNodes.All = append(kafkaNodes.All, instance.PrivateIpAddress)
				}
				if tag["Key"] == "Component" && tag["Value"] == "schemaRegistry" {
					kafkaNodes.SchemaRegistry = append(kafkaNodes.SchemaRegistry, instance.PrivateIpAddress)
					kafkaNodes.All = append(kafkaNodes.All, instance.PrivateIpAddress)
				}

			}
		}

	}

	commands := []string{
		"sudo apt-get update -y 1>/dev/null",
		"sudo apt-get install -y gnupg2 software-properties-common openjdk-11-jdk jq 1>/dev/null 2>/dev/null",
		"wget https://packages.confluent.io/deb/7.1/archive.key -P /tmp/",
		"sudo apt-key add /tmp/archive.key",
		`sudo add-apt-repository 'deb [arch=amd64] https://packages.confluent.io/deb/7.1 stable main'`,
		`sudo add-apt-repository 'deb https://packages.confluent.io/clients/deb '$(lsb_release -cs)' main'`,
		"sudo apt-get update -y 1>/dev/null",
		"sudo apt-get install -y confluent-platform confluent-security confluent-community-2.13  1>/dev/null 2>/dev/null",
	}

	for _, cmd := range commands {
		if _, _, err := (*workstation).Execute(ctx, cmd, false, 600*time.Second); err != nil {
			return err
		}
	}

	if _, _, err := (*workstation).Execute(ctx, "mkdir -p /opt/kafka/perf", true); err != nil {
		return err
	}

	for _, file := range []string{"kafka.create.topic.sh", "kafka.producer.perf.sh", "kafka.consumer.perf.sh", "kafka.e2e.perf.sh"} {
		fmt.Printf("The template file to parse <%s> \n\n\n", file)
		err = (*workstation).TransferTemplate(ctx, fmt.Sprintf("templates/config/%s.tpl", file), fmt.Sprintf("/tmp/%s", file), "0755", kafkaNodes, true, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("mv /tmp/%s /opt/kafka/perf/%s", file, file), true); err != nil {
			return err
		}
	}
	fmt.Printf("The template file was completed \n\n\n  ")

	for _, node := range kafkaNodes.All {
		fmt.Printf("The data is <%s> \n\n\n\n", node)
		for _, cmd := range commands {
			if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, node, cmd), false, 600*time.Second); err != nil {
				return err
			}
		}

	}

	err = (*workstation).TransferTemplate(ctx, "templates/config/zookeeper.properties.tpl", "/tmp/zookeeper.properties", "0644", kafkaNodes, true, 0)
	if err != nil {
		return err
	}

	for idx, node := range kafkaNodes.Zookeeper {
		commands = []string{
			"sudo mv /tmp/zookeeper.properties /etc/kafka/zookeeper.properties",
			"mkdir -p /tmp/zookeeper/data; mkdir -p /tmp/zookeeper/logs; sudo chown -R cp-kafka /tmp/zookeeper",
			"sudo rm -f /tmp/zookeeper/data/myid",
			fmt.Sprintf("echo %d | sudo tee -a /tmp/zookeeper/data/myid; sudo chown cp-kafka /tmp/zookeeper/data/myid", idx),
			"sudo systemctl restart confluent-zookeeper",
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /tmp/zookeeper.properties %s:/tmp/zookeeper.properties`, node), false); err != nil {
			return err
		}

		for _, cmd := range commands {

			if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, node, cmd), false, 600*time.Second); err != nil {
				return err
			}
		}

	}

	type BrokerData struct {
		Zookeeper []string
		BrokerID  int
		BrokerIP  string
	}

	for idx, node := range kafkaNodes.Broker {
		commands = []string{
			"sudo mv /etc/kafka/server.properties /etc/kafka/server.properties.bak",
			"sudo mv /tmp/kafka.server.properties /etc/kafka/server.properties",
			"sudo systemctl restart confluent-kafka",
		}

		var brokerData BrokerData
		brokerData.Zookeeper = kafkaNodes.Zookeeper
		brokerData.BrokerID = idx
		brokerData.BrokerIP = node
		err = (*workstation).TransferTemplate(ctx, "templates/config/kafka.server.properties.tpl", "/tmp/kafka.server.properties", "0644", brokerData, true, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /tmp/kafka.server.properties %s:/tmp/kafka.server.properties`, node), false); err != nil {
			return err
		}

		for _, cmd := range commands {

			if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, node, cmd), false, 600*time.Second); err != nil {
				return err
			}
		}

	}

	for _, node := range kafkaNodes.SchemaRegistry {
		commands = []string{
			"sudo mv /etc/schema-registry/schema-registry.properties /etc/schema-registry/schema-registry.properties.bak",
			"sudo mv /tmp/kafka.schema-registry.properties /etc/schema-registry/schema-registry.properties",
			"sudo systemctl restart confluent-schema-registry",
		}

		err = (*workstation).TransferTemplate(ctx, "templates/config/kafka.schema-registry.properties.tpl", "/tmp/kafka.schema-registry.properties", "0644", kafkaNodes, true, 0)
		if err != nil {
			return err
		}

		if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`scp /tmp/kafka.schema-registry.properties %s:/tmp/kafka.schema-registry.properties`, node), false); err != nil {
			return err
		}

		for _, cmd := range commands {

			if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, node, cmd), false, 600*time.Second); err != nil {
				return err
			}
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployKafka) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployKafka) String() string {
	return fmt.Sprintf("Echo: Deploying Kafka")
}
