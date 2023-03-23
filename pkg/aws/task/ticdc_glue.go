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

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	// "github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

type BaseTiCDCGlue struct {
	BaseTask
}

func (b *BaseTiCDCGlue) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	return nil
}

type CreateTiCDCGlue struct {
	BaseTiCDCGlue
}

// Execute implements the Task interface
func (c *CreateTiCDCGlue) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	log.Infof("***** CreateTiCDCGlueCluster ****** \n\n\n")

	if _, _, err := (*c.wsExe).Execute(context.Background(), "rm -f cdc.x86_64.zip && wget https://github.com/luyomo/tiflow-glue/releases/download/glue/cdc.x86_64.zip", false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), "rm -f cdc && unzip cdc.x86_64.zip", false); err != nil {
		return err
	}

	cdcInstances, err := c.getTiDBComponent("cdc")
	if err != nil {
		return err
	}

	for _, instance := range *cdcInstances {
		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("/home/admin/.tiup/bin/tiup cluster stop -y %s --node %s", c.clusterName, instance.ID), false); err != nil {
			return err
		}

		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("scp -o  StrictHostKeyChecking=no cdc %s:%s/bin", instance.Host, instance.DeployDir), false); err != nil {
			return err
		}

		c.syncCDCServiceFile(instance.Host)

		if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("/home/admin/.tiup/bin/tiup cluster start -y %s --node %s", c.clusterName, instance.ID), false); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTiCDCGlue) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiCDCGlue) String() string {
	return fmt.Sprintf("Echo: Create TiCDC Glue  ... ...  ")
}

func (c *CreateTiCDCGlue) syncCDCServiceFile(cdcIP string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	serviceFileENV := make(map[string]string)
	crentials, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		return err
	}

	serviceFileENV["AWS_REGION"] = cfg.Region
	serviceFileENV["AWS_ACCESS_KEY_ID"] = crentials.AccessKeyID
	serviceFileENV["AWS_SECRET_ACCESS_KEY"] = crentials.SecretAccessKey

	if err = (*c.wsExe).TransferTemplate(context.Background(), "templates/systemd/cdc.awsenv.service.tpl", "/tmp/cdc-8300.service", "0644", serviceFileENV, false, 0); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("scp -o StrictHostKeyChecking=no /tmp/cdc-8300.service %s:/tmp/", cdcIP), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("ssh %s 'sudo mv /tmp/cdc-8300.service /etc/systemd/system/cdc-8300.service'", cdcIP), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("ssh %s 'sudo systemctl daemon-reload'", cdcIP), false); err != nil {
		return err
	}

	return nil
}
