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
	//	"errors"
	"fmt"
	//"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	//	"github.com/luyomo/OhMyTiUP/pkg/executor"
	//	"go.uber.org/zap"
	"strings"
	"time"
)

type CreateDMSSourceEndpoint struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSSourceEndpoint) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws dms describe-endpoints --filters Name=endpoint-type,Values=source Name=engine-name,Values=aurora")
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), "No Endpoints found matching provided filters") {
			return err
		}
	} else {
		var endpoints Endpoints
		if err = json.Unmarshal(stdout, &endpoints); err != nil {
			return nil
		}
		for _, endpoint := range endpoints.Endpoints {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, endpoint.EndpointArn, *c.pexecutor, ctx)
			if existsResource == true {
				DMSInfo.SourceEndpointArn = endpoint.EndpointArn
				return nil
			}
		}
	}

	dbInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "aurora")
	if err != nil {
		return err
	}

	command = fmt.Sprintf("aws dms create-endpoint --endpoint-identifier %s-source --endpoint-type source --engine-name aurora --server-name %s --port %d --username %s --password %s --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, dbInstance.Endpoint.Address, dbInstance.Endpoint.Port, dbInstance.MasterUsername, "1234Abcd", clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var endpoint EndpointRecord
	if err = json.Unmarshal(stdout, &endpoint); err != nil {
		return err
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
	return fmt.Sprintf("Echo: Creating DMS Source Endpoints")
}

/******************************************************************************/

type CreateDMSTargetEndpoint struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMSTargetEndpoint) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws dms describe-endpoints --filters Name=endpoint-type,Values=target Name=engine-name,Values=sqlserver")
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		if !strings.Contains(string(stderr), "No Endpoints found matching provided filters") {
			return err
		}
	} else {
		var endpoints Endpoints
		if err = json.Unmarshal(stdout, &endpoints); err != nil {
			return err
		}
		for _, endpoint := range endpoints.Endpoints {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, endpoint.EndpointArn, *c.pexecutor, ctx)
			if existsResource == true {
				DMSInfo.TargetEndpointArn = endpoint.EndpointArn
				return nil
			}
		}
	}

	dbInstance, err := getRDBInstance(*c.pexecutor, ctx, clusterName, clusterType, "sqlserver")
	if err != nil {
		return err
	}

	command = fmt.Sprintf("aws dms create-endpoint --endpoint-identifier %s-target --endpoint-type target --engine-name sqlserver --server-name %s --port %d --username %s --password %s --database-name %s --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, dbInstance.Endpoint.Address, dbInstance.Endpoint.Port, dbInstance.MasterUsername, "1234Abcd", "cdc_test", clusterName, clusterType, c.subClusterType)
	stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var endpoint EndpointRecord
	if err = json.Unmarshal(stdout, &endpoint); err != nil {
		return err
	}
	DMSInfo.TargetEndpointArn = endpoint.Endpoint.EndpointArn

	return nil
}

// Rollback implements the Task interface
func (c *CreateDMSTargetEndpoint) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMSTargetEndpoint) String() string {
	return fmt.Sprintf("Echo: Creating DMS target endpoint")
}

/******************************************************************************/

type DestroyDMSEndpoints struct {
	pexecutor      *ctxt.Executor
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyDMSEndpoints) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws dms describe-endpoints")
	stdout, stderr, err := (*c.pexecutor).Execute(ctx, command, false)
	var deletingEndpoints []string
	if err != nil {
		if !strings.Contains(string(stderr), "No Endpoints found matching provided filters") {
			return err
		}
	} else {
		var endpoints Endpoints
		if err = json.Unmarshal(stdout, &endpoints); err != nil {
			return err
		}
		for _, endpoint := range endpoints.Endpoints {
			existsResource := ExistsDMSResource(clusterType, c.subClusterType, clusterName, endpoint.EndpointArn, *c.pexecutor, ctx)
			if existsResource == true {
				command = fmt.Sprintf("aws dms delete-endpoint --endpoint-arn %s", endpoint.EndpointArn)

				stdout, stderr, err = (*c.pexecutor).Execute(ctx, command, false)

				if err != nil {
					return err
				}
				deletingEndpoints = append(deletingEndpoints, endpoint.EndpointArn)
			}
		}
	}

	command = fmt.Sprintf("aws dms describe-endpoints")
	for i := 1; i <= 50; i++ {
		cntEndpoints := 0

		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		} else {
			var endpoints Endpoints
			if err = json.Unmarshal(stdout, &endpoints); err != nil {
				return err
			}
			for _, endpoint := range endpoints.Endpoints {
				for _, arn := range deletingEndpoints {
					if arn == endpoint.EndpointArn {
						cntEndpoints++
					}
				}
			}
			if cntEndpoints == 0 {
				break
			}
		}

		time.Sleep(30 * time.Second)
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyDMSEndpoints) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyDMSEndpoints) String() string {
	return fmt.Sprintf("Echo: Destroying DMS endpoints")
}
