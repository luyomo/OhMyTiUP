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
	// cancelCtx *context.CancelFunc
	// metricsOfLatencyWhenBatch *MetricsOfLatencyWhenBatch
}

// Execute implements the Task interface
func (c *RunOntimeBatchInsert) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	fmt.Printf("The batch size is <%d> \n\n\n\n\n", (*(c.opt)).BatchSize)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, (*(c.gOpt)).SSHUser, (*(c.gOpt)).IdentityFile)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration((*c.opt).TransInterval) * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			// startTime := time.Now()
			_, _, err := (*workstation).Execute(context.Background(), fmt.Sprintf(`/opt/scripts/ontime_batch_insert.sh latencytest ontime01 ontime %d`, (*(c.opt)).BatchSize), false, 5*time.Hour)
			// elapsedTime := time.Since(startTime) / time.Millisecond

			// fmt.Printf("The lapsed time is  <%d> \n\n\n\n\n\n\n", elapsedTime)
			if err != nil {
				return err
			}

			// (*c.metricsOfLatencyWhenBatch).TransRow++

			// (*c.metricsOfLatencyWhenBatch).TotalExecutionTime += int64(elapsedTime)
		case <-ctx.Done():
			// (*c.metricsOfLatencyWhenBatch).AverageExecutionTime = (*c.metricsOfLatencyWhenBatch).TotalExecutionTime / (*c.metricsOfLatencyWhenBatch).TransRow
			// (*c.metricsOfLatencyWhenBatch).TransRow++

			return nil
		}

	}

	return nil

	// (*c.metricsOfLatencyWhenBatch).Loop = (*(c.opt)).BatchLoop
	// (*c.metricsOfLatencyWhenBatch).BatchSize = (*(c.opt)).BatchSize
	// (*c.metricsOfLatencyWhenBatch).BatchTotalRows = int64((*c.metricsOfLatencyWhenBatch).Loop) * int64((*c.metricsOfLatencyWhenBatch).BatchSize)

	// // 1. Get all the workstation nodes
	// workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, (*(c.gOpt)).SSHUser, (*(c.gOpt)).IdentityFile)
	// if err != nil {
	// 	return err
	// }

	// startTime := time.Now()
	// //fmt.Printf("The command is <%s>", fmt.Sprintf(`/opt/scripts/ontime_batch_insert.sh latencytest ontime01 ontime %d %d`, (*(c.opt)).BatchLoop, (*(c.opt)).BatchSize))
	// _, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`/opt/scripts/ontime_batch_insert.sh latencytest ontime01 ontime %d %d`, (*(c.opt)).BatchLoop, (*(c.opt)).BatchSize), false, 5*time.Hour)
	// (*c.metricsOfLatencyWhenBatch).BatchExecutionTime = int64(time.Since(startTime) / time.Millisecond)

	// if err != nil {
	// 	(*c.cancelCtx)()
	// 	return err
	// }
	// (*c.cancelCtx)()

	// return nil
}

// Rollback implements the Task interface
func (c *RunOntimeBatchInsert) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RunOntimeBatchInsert) String() string {
	return fmt.Sprintf("Echo: Running batch insert")
}

// ----- ----- ----- ------ ----- PntimeInsert
type RunOntimeTpInsert struct {
	pexecutor                 *ctxt.Executor
	gOpt                      *operator.Options
	opt                       *operator.LatencyWhenBatchOptions
	metricsOfLatencyWhenBatch *MetricsOfLatencyWhenBatch
}

// Execute implements the Task interface
func (c *RunOntimeTpInsert) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, (*(c.gOpt)).SSHUser, (*(c.gOpt)).IdentityFile)
	if err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, `/opt/scripts/run_tidb_query latencytest "delete from test01"`, false, 5*time.Hour); err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration((*c.opt).TransInterval) * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			startTime := time.Now()
			_, _, err = (*workstation).Execute(context.Background(), `/opt/scripts/run_tidb_query latencytest "insert into test01(col02, col03) values(1, 'This is the test message')"`, false, 5*time.Hour)
			elapsedTime := time.Since(startTime) / time.Millisecond

			// fmt.Printf("The lapsed time is  <%d> \n\n\n\n\n\n\n", elapsedTime)
			if err != nil {
				return err
			}

			(*c.metricsOfLatencyWhenBatch).TransRow++

			(*c.metricsOfLatencyWhenBatch).TotalExecutionTime += int64(elapsedTime)
		case <-ctx.Done():
			(*c.metricsOfLatencyWhenBatch).AverageExecutionTime = (*c.metricsOfLatencyWhenBatch).TotalExecutionTime / (*c.metricsOfLatencyWhenBatch).TransRow
			(*c.metricsOfLatencyWhenBatch).TransRow++

			return nil
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *RunOntimeTpInsert) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RunOntimeTpInsert) String() string {
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

	// if _, _, err = (*workstation).Execute(ctx, `/opt/scripts/run_tidb_query latencytest "delete from test01"`, false, 5*time.Hour); err != nil {
	// 	return err
	// }

	// ticker := time.NewTicker(time.Duration((*c.opt).TransInterval) * time.Millisecond)

	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		startTime := time.Now()
	// 		_, _, err = (*workstation).Execute(context.Background(), `/opt/scripts/run_tidb_query latencytest "insert into test01(col02, col03) values(1, 'This is the test message')"`, false, 5*time.Hour)
	// 		elapsedTime := time.Since(startTime) / time.Millisecond

	// 		// fmt.Printf("The lapsed time is  <%d> \n\n\n\n\n\n\n", elapsedTime)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		(*c.metricsOfLatencyWhenBatch).TransRow++

	// 		(*c.metricsOfLatencyWhenBatch).TotalExecutionTime += int64(elapsedTime)
	// 	case <-ctx.Done():
	// 		(*c.metricsOfLatencyWhenBatch).AverageExecutionTime = (*c.metricsOfLatencyWhenBatch).TotalExecutionTime / (*c.metricsOfLatencyWhenBatch).TransRow
	// 		(*c.metricsOfLatencyWhenBatch).TransRow++

	// 		return nil
	// 	}

	// }

	fmt.Printf("Starting to run the sysbench ... ... ... ... \n\n\n")
	//
	stdout, _, err := (*workstation).Execute(context.Background(), `sysbench --config-file=/opt/sysbench.toml tidb_oltp_insert_simple --tables=8 --table-size=100000 run`, false, 5*time.Hour)

	if err != nil {
		return err
	}
	fmt.Printf("The output from the sysbench is <%s> \n\n\n", stdout)
	arrLines := strings.Split(string(stdout), "\n")
	// fmt.Printf("The output is <%#v> \n\n\n", arrLines)
	isOutput := false
	skipLine := false
	for _, line := range arrLines {
		if line == "---------- Result summary ----------" {
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
			if skipLine == true {
				skipLine = false
				continue
			}

			*c.sysbenchResult = append(*c.sysbenchResult, strings.Split(line, ","))
			// fmt.Printf("The data is : %s \n\n\n", line)
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
