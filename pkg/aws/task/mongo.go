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
	// "encoding/json"
	"fmt"
	// "strconv"
	"strings"
	"time"

	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	// "go.uber.org/zap"
)

type DeployMongo struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

type (
	Member struct {
		ID   int    `json:"_id"`
		Host string `json:"host"`
	}

	RSConfig struct {
		ID      string   `json:"_id"`
		Members []Member `json:"members"`
	}

	RSStatus struct {
		ReplicaSet string `json:"set"`
		MyState    int    `json:"myState"`
		Members    []struct {
			ID       int    `json:"_id"`
			Name     string `json:"name"`
			StateStr string `json:"stateStr"`
		} `json:"members"`
	}
)

// Execute implements the Task interface
func (c *DeployMongo) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	fmt.Printf("Process reach the execute of mongo deployment \n\n\n")

	/* ********** ********** 001. Prepare execution context  **********/
	// 001.01. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, `apt-get update`, true)
	if err != nil {
		return err
	}

	_, _, err = (*workstation).Execute(ctx, `apt-get install rsync`, true)
	if err != nil {
		return err
	}

	var rsConfig RSConfig
	rsConfig.ID = "rs0"

	_ec2List, err := getEC2Nodes(*c.pexecutor, ctx, clusterName, clusterType, "replicaSet")
	var pkgInstallTasks []Task
	var listMongoIP []string
	var proxyIP string
	for _idx, _ec2Node := range *_ec2List {
		pkgInstallTask := &MongoInstallPkgTask{
			wsexecutor: workstation,
			exeNode:    _ec2Node.PrivateIpAddress,
		}

		pkgInstallTasks = append(pkgInstallTasks, pkgInstallTask)

		var member Member
		member.ID = _idx
		member.Host = _ec2Node.PrivateIpAddress
		rsConfig.Members = append(rsConfig.Members, member)

		// Get the IP address to execute the command against mongo cluster
		if _idx == 0 {
			proxyIP = _ec2Node.PrivateIpAddress
		} else {
			listMongoIP = append(listMongoIP, _ec2Node.PrivateIpAddress)
		}
	}

	// fmt.Printf("The host is <%#v> \n\n\n", rsConfig)
	// _rsJsonConfig, err := json.Marshal(rsConfig)
	// if err != nil {
	// 	return err
	// }
	// _strJsonConfig := string(_rsJsonConfig)
	// fmt.Println(_strJsonConfig)

	parallelExe := Parallel{ignoreError: false, inner: pkgInstallTasks}
	if err := parallelExe.Execute(ctx); err != nil {
		return err
	}

	for _idx := 0; _idx < 10; _idx++ {
		_, stderr, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, proxyIP, "mongosh --quiet --eval 'rs.status().members.filter(function(rsStatus) { return rsStatus.state === 1;}).length'"), false, 600*time.Second)
		if strings.Contains(string(stderr), "no replset config has been received") {
			_, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s 'mongosh --quiet --eval "rs.initiate()"'`, proxyIP), false, 600*time.Second)
			if err != nil {
				return err
			}
			for _, _memberIP := range listMongoIP {
				_, _, err := (*workstation).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s 'mongosh --quiet --eval "rs.add(\"%s:27017\")"'`, proxyIP, _memberIP), false, 600*time.Second)
				if err != nil {
					return err
				}
			}
			break
			fmt.Printf("01. Run the script to initialize the replica set \n\n\n")
		} else if strings.Contains(string(stderr), "MongoServerError: not running with --replSet") {
			print("Reaching the not running with --replSet\n\n\n")
			time.Sleep(10 * time.Second)
			continue

		} else if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployMongo) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployMongo) String() string {
	return fmt.Sprintf("Echo: Deploying Kafka")
}

// *********** The package installation for parallel
type MongoInstallPkgTask struct {
	wsexecutor *ctxt.Executor
	exeNode    string
}

func (c *MongoInstallPkgTask) Execute(ctx context.Context) error {
	type MongoConf struct {
		ReplicaSet string
		IPAddress  string
	}

	commands := []string{
		"sudo apt-get -y update",
		"sudo apt-get install -y gnupg rsync",
		"wget -qO - https://www.mongodb.org/static/pgp/server-6.0.asc | sudo apt-key add -",
		"echo 'deb http://repo.mongodb.org/apt/debian buster/mongodb-org/6.0 main' | sudo tee /etc/apt/sources.list.d/mongodb-org-6.0.list",
		"sudo apt-get -y update",
		"sudo apt-get install -y mongodb-org",
	}

	for _, cmd := range commands {
		if _, _, err := (*(c.wsexecutor)).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, c.exeNode, cmd), false, 600*time.Second); err != nil {
			return err
		}
	}

	err := (*c.wsexecutor).TransferTemplate(ctx, "templates/config/tidb2kafka2mongo/mongod.tpl.conf", fmt.Sprintf("/tmp/mongod.%s.conf", c.exeNode), "0644", MongoConf{ReplicaSet: "rs0", IPAddress: c.exeNode}, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*(c.wsexecutor)).Execute(ctx, fmt.Sprintf(`rsync /tmp/mongod.%s.conf %s:/tmp/mongod.conf`, c.exeNode, c.exeNode), false, 600*time.Second); err != nil {
		return err
	}

	if _, _, err := (*(c.wsexecutor)).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, c.exeNode, "sudo mv /tmp/mongod.conf /etc"), false, 600*time.Second); err != nil {
		return err
	}

	if _, _, err := (*(c.wsexecutor)).Execute(ctx, fmt.Sprintf(`ssh -o "StrictHostKeyChecking no" %s "%s"`, c.exeNode, "sudo systemctl restart mongod"), false, 600*time.Second); err != nil {
		return err
	}

	return nil
}
func (c *MongoInstallPkgTask) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

func (c *MongoInstallPkgTask) String() string {
	return fmt.Sprintf("Echo: Parallel kafka package install")
}
