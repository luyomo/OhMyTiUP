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
	//"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"go.uber.org/zap"
	"strings"
	//"time"
)

type Endpoint struct {
	EndpointIdentifier string `json:"EndpointIdentifier"`
	EndpointType       string `json:"EndpointType"`
	EngineName         string `json:"EngineName"`
	Status             string `json:"Status"`
	EndpointArn        string `json:"EndpointArn"`
}

type Endpoints struct {
	Endpoints []Endpoint `json:"Endpoints"`
}

type CreateDMSSourceEndpoint struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *CreateDMSSourceEndpoint) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	command := fmt.Sprintf("aws dms describe-endpoints --filters Name=endpoint-type,Values=source Name=engine-name,Values=aurora")
	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		if strings.Contains(string(stderr), "No Endpoints found matching provided filters") {
			fmt.Printf("The source end point has not created.\n\n\n")
		} else {
			fmt.Printf("The error err here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
			return nil
		}
	} else {
		var endpoints Endpoints
		if err = json.Unmarshal(stdout, &endpoints); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil
		}
		fmt.Printf("The db cluster is <%#v> \n\n\n", endpoints)
		for _, endpoint := range endpoints.Endpoints {
			existsResource := ExistsDMSResource(c.clusterType, c.clusterName, endpoint.EndpointArn, local, ctx)
			if existsResource == true {
				fmt.Printf("The dms source has exists \n\n\n")
				return nil
			}
		}
	}

	command = fmt.Sprintf("aws dms create-endpoint --endpoint-identifier %s --endpoint-type source --engine-name aurora --server-name testtisample.ckcbeq0sbqxz.ap-northeast-1.rds.amazonaws.com --port 3306 --username master --password 1234Abcd --tags Key=Name,Value=%s Key=Type,Value=%s", c.clusterName, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSSourceEndpoint) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSSourceEndpoint) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

func ExistsDMSResource(clusterType, clusterName, resourceName string, executor ctxt.Executor, ctx context.Context) bool {
	command := fmt.Sprintf("aws dms list-tags-for-resource --resource-arn %s ", resourceName)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return false
	}

	var tagList TagList
	if err = json.Unmarshal(stdout, &tagList); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return false
	}
	matchedCnt := 0
	for _, tag := range tagList.TagList {
		if tag.Key == "Type" && tag.Value == clusterType {
			matchedCnt++
		}
		if tag.Key == "Name" && tag.Value == clusterName {
			matchedCnt++
		}
		if matchedCnt == 2 {
			return true
		}
	}
	return false
}
