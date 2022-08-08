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
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/ctxt"
	"strings"
	"time"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"github.com/luyomo/tisample/pkg/aws/spec"
	// "github.com/aws/aws-sdk-go-v2/aws"
	// "github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "go.uber.org/zap"
	// "strings"
	//	"time"
)

type MetricsOfLatencyWhenBatch struct {
	TransRow             int64
	TotalExecutionTime   int64
	AverageExecutionTime int64

	BatchExecutionTime int64
	BatchSize          int
	Loop               int
	BatchTotalRows     int64
}

type RunOntimeBatchInsert struct {
	pexecutor *ctxt.Executor
	gOpt      *operator.Options
	opt       *operator.LatencyWhenBatchOptions
}

// Execute implements the Task interface
func (c *RunOntimeBatchInsert) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, (*(c.gOpt)).SSHUser, (*(c.gOpt)).IdentityFile)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration((*c.opt).TransInterval) * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			_, _, err := (*workstation).Execute(context.Background(), fmt.Sprintf(`/opt/scripts/ontime_batch_insert.sh latencytest ontime01 ontime %d`, (*(c.opt)).BatchSize), false, 5*time.Hour)

			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *RunOntimeBatchInsert) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RunOntimeBatchInsert) String() string {
	return fmt.Sprintf("Echo: Running batch insert")
}

// ------ ----- ----- RunSysbench
type RunSysbench struct {
	pexecutor *ctxt.Executor
	gOpt      *operator.Options
	opt       *operator.LatencyWhenBatchOptions

	sysbenchResult *[][]string
	cancelCtx      *context.CancelFunc
}

// Execute implements the Task interface
func (c *RunSysbench) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, (*(c.gOpt)).SSHUser, (*(c.gOpt)).IdentityFile)
	if err != nil {
		return err
	}

	stdout, _, err := (*workstation).Execute(context.Background(), fmt.Sprintf(`sysbench --config-file=%s %s --tables=%d --table-size=%d run`, "/opt/aurora-sysbench.toml", (*c.opt).SysbenchPluginName, (*c.opt).SysbenchNumTables, (*c.opt).SysbenchNumRows), false, 5*time.Hour)

	if err != nil {
		return err
	}

	arrLines := strings.Split(string(stdout), "\n")

	isOutput := false
	skipLine := false
	for _, line := range arrLines {
		if line == "---------- Result summary ----------" {
			// If there is no data in the array, need to add the header to the table GUI
			if len(*c.sysbenchResult) > 0 {
				skipLine = true
			}
			isOutput = true
			continue
		}

		if line == "---------- End result summary ----------" {
			break
		}

		if isOutput == true {
			// If the skipLine is true, skip the first line. Only run one time if there is no data.
			if skipLine == true {
				skipLine = false
				continue
			}

			arrData := strings.Split(line, ",")
			if len(*c.sysbenchResult) > 0 {
				arrData = append([]string{fmt.Sprintf("%d", c.opt.BatchSize)}, arrData...)
				*c.sysbenchResult = append(*c.sysbenchResult, arrData)
			} else {
				arrData = append([]string{"Batch size"}, arrData...)
				*c.sysbenchResult = append(*c.sysbenchResult, arrData)
			}
		}
	}

	if err != nil {
		(*c.cancelCtx)()
		return err
	}
	(*c.cancelCtx)()

	return nil
}

// Rollback implements the Task interface
func (c *RunSysbench) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RunSysbench) String() string {
	return fmt.Sprintf("Echo: Running batch insert")
}
