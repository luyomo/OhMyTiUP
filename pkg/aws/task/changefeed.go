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
	// "errors"
	"fmt"
	// "gopkg.in/yaml.v3"
	// "io/ioutil"
	// "strings"
	"encoding/json"
	"math/big"

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

/******************************************************************************/
func (b *Builder) CreateChangefeed(wsExe *ctxt.Executor, mskEndpoints *string) *Builder {
	b.tasks = append(b.tasks, &CreateChangefeed{BaseChangefeed: BaseChangefeed{
		BaseTask:     BaseTask{wsExe: wsExe},
		mskEndpoints: mskEndpoints,
	}})
	return b
}

/******************************************************************************/
// Struct definition
type ChangefeedInfo struct {
	ID        string `json:"id,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Summary   struct {
		State      string  `json:"state,omitempty"`
		Tso        big.Int `json:"tso,omitempty"`
		Checkpoint string  `json:"checkpoint,omitempty"`
		Error      string  `json:"error,omitempty"`
	} `json:"summary"`
}

/******************************************************************************/

type BaseChangefeed struct {
	BaseTask

	mskEndpoints *string
}

func (b *BaseChangefeed) init(ctx context.Context) error {
	b.clusterName = ctx.Value("clusterName").(string)
	b.clusterType = ctx.Value("clusterType").(string)

	return nil
}

func (b *BaseChangefeed) listChangefeeds() (*[]ChangefeedInfo, error) {
	cdcInstances, err := b.getTiDBComponent("cdc")
	if err != nil {
		return nil, err
	}

	stdout, _, err := (*b.wsExe).Execute(context.Background(), fmt.Sprintf(`~/cdc cli changefeed list --server http://%s `, (*cdcInstances)[0].ID), false)
	if err != nil {
		return nil, err
	}

	var changefeeds []ChangefeedInfo
	if err = json.Unmarshal(stdout, &changefeeds); err != nil {
		return nil, err
	}

	return &changefeeds, nil

}

func (b *BaseChangefeed) changefeedExist(changefeedID string) (bool, error) {
	changefeeds, err := b.listChangefeeds()
	if err != nil {
		return false, err
	}

	for _, changefeed := range *changefeeds {
		if changefeed.ID == changefeedID {
			return true, nil
		}
	}

	return false, nil
}

type CreateChangefeed struct {
	BaseChangefeed
}

/*
   /home/admin/.tiup/bin/tiup cdc cli changefeed list --server 182.83.6.71:8300

   /home/admin/.tiup/bin/tiup cdc cli changefeed create --server http://%s:8300 --changefeed-id='%s' --sink-uri='kafka://%s:9092/%s?protocol=avro' --schema-registry=http://%s:8081 --config %s", cdcIP, "kafka-avro", brokerIP, "topic-name", schemaRegistryIP, "/opt/kafka/source.toml"), false); err !=

   1. cdc server
   2. glue schema registry
   3. kafka url
   4. Config file
*/

// Execute implements the Task interface
func (c *CreateChangefeed) Execute(ctx context.Context) error {

	log.Infof("***** CreateChangefeed ****** \n\n\n")

	c.init(ctx)

	changefeeds, err := c.listChangefeeds()
	if err != nil {
		return err
	}
	fmt.Printf("The changefeeds are <%#v> \n\n\n\n\n\n", changefeeds)

	changefeedExist, err := c.changefeedExist(c.clusterName)
	if err != nil {
		return err
	}

	if changefeedExist == true {
		return nil
	}

	cdcInstances, err := c.getTiDBComponent("cdc")
	if err != nil {
		return err
	}
	fmt.Printf("Fetch the cdc instance : <%#v> \n\n\n\n\n\n", cdcInstances)
	createChangefeedCmd := fmt.Sprintf(`~/cdc cli changefeed create --server http://%s --changefeed-id=%s --sink-uri="kafka://%s/topic-name?protocol=avro&replication-factor=3" --schema-registry=%s --schema-registry-provider=glue --config /opt/kafka/source.toml`, (*cdcInstances)[0].ID, c.clusterName, *c.mskEndpoints, c.clusterName)

	fmt.Printf("command: %s", createChangefeedCmd)

	return nil
}

// Rollback implements the Task interface
func (c *CreateChangefeed) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateChangefeed) String() string {
	return fmt.Sprintf("Echo: Create changefeed  ... ...  ")
}
