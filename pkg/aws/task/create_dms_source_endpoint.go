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
	//	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"go.uber.org/zap"
	"strings"
)

type CreateDMSSourceEndpoint struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
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
			existsResource := ExistsDMSResource(c.clusterType, c.subClusterType, c.clusterName, endpoint.EndpointArn, local, ctx)
			if existsResource == true {
				DMSInfo.SourceEndpointArn = endpoint.EndpointArn
				fmt.Printf("The dms source has exists \n\n\n")
				return nil
			}
		}
	}

	dbInstance, err := getRDBInstance(local, ctx, c.clusterName, c.clusterType, "aurora")
	if err != nil {
		fmt.Printf("The error is <%#v> \n\n\n", dbInstance)
		return err
	}
	fmt.Printf("The dn instance is <%#v> \n\n\n", dbInstance)

	command = fmt.Sprintf("aws dms create-endpoint --endpoint-identifier %s-source --endpoint-type source --engine-name aurora --server-name %s --port %d --username %s --password %s --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", c.clusterName, dbInstance.Endpoint.Address, dbInstance.Endpoint.Port, dbInstance.MasterUsername, "1234Abcd", c.clusterName, c.clusterType, c.subClusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}

	var endpoint EndpointRecord
	if err = json.Unmarshal(stdout, &endpoint); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	DMSInfo.SourceEndpointArn = endpoint.Endpoint.EndpointArn

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
