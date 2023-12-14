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
	"strings"
	"time"

	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

func (b *Builder) RunOntimeBatchInsert(workstation *ws.Workstation, opt *operator.LatencyWhenBatchOptions, dumplingCnt *int64) *Builder {
	b.tasks = append(b.tasks, &RunOntimeBatchInsert{
		opt:         opt,
		dumplingCnt: dumplingCnt,
		workstation: workstation,
	})
	return b
}

func (b *Builder) RunSysbench(pexecutor *ctxt.Executor, sysbenchConfigFile string, sysbenchResult *[][]string, opt *operator.LatencyWhenBatchOptions, cancelCtx *context.CancelFunc) *Builder {
	b.tasks = append(b.tasks, &RunSysbench{
		pexecutor:          pexecutor,
		opt:                opt,
		sysbenchConfigFile: sysbenchConfigFile,
		sysbenchResult:     sysbenchResult,
		cancelCtx:          cancelCtx,
	})
	return b
}

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
	pexecutor   *ctxt.Executor
	workstation *ws.Workstation
	opt         *operator.LatencyWhenBatchOptions
	dumplingCnt *int64
}

// Bckground: Test the resource control's effect. The process are simulated. One the online application by tpcc while another is batch by insert...select query. Use resource control to limit the tpcc's latency and qps impact from batch.
// Description: There are seveal modes to call the batch process:
//  01. partition:
//  02. batch: Split the big transaction into several small transactions without user's source hacker.  https://docs.pingcap.com/tidb/stable/sql-statement-batch
//  03. insert: User the insert into ontime (...) select ... from ontime01
//  04. dumpling: Dumpling the data from ontime01 to similate the batch process
//
// For dumpling mode, count the ontime01 first and calculate the sum export by ontime01's count * idx(number of loop)
// For non-dumpling mode, count the ontime before and after the batch to diff out the count of the batch
func (c *RunOntimeBatchInsert) Execute(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration((*c.opt).TransInterval) * time.Millisecond)

	var cntOntime01 int64
	if c.opt.BatchMode == "dumpling" {
		_data, err := c.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime01")
		if err != nil {
			return err
		}
		cntOntime01 = int64((*_data)[0]["ontime_cnt"].(float64))
	} else {
		_data, err := c.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
		if err != nil {
			return err
		}
		cntOntime01 = int64((*_data)[0]["ontime_cnt"].(float64))
	}

	idx := 0
	for {
		select {
		case <-ctx.Done(): // Signal from another thread that it has completed.
			fmt.Printf("Rows are inserted into batch table: %d and %d \n\n\n\n\n\n", idx, (*(c.opt)).BatchSize)

			if c.opt.BatchMode == "dumpling" {
				*(c.dumplingCnt) = int64(idx) * cntOntime01
			} else {
				_data, err := c.workstation.QueryTiDB("test", "select count(*) ontime_cnt from latencytest.ontime")
				if err != nil {
					return err
				}
				curOntimeCnt := int64((*_data)[0]["ontime_cnt"].(float64))
				*(c.dumplingCnt) = curOntimeCnt - cntOntime01
			}

			return nil
		case <-ticker.C:
			// fmt.Printf("Starting to copy data: %d \n\n\n", idx)
			command := ""
			// insert / batch / partition
			switch c.opt.BatchMode {
			case "partition":
				command = fmt.Sprintf(`/opt/scripts/ontime_shard_batch_insert.sh latencytest ontime01 ontime %s`, c.opt.BatchMode)
			case "batch":
				command = fmt.Sprintf(`/opt/scripts/ontime_shard_batch_insert.sh latencytest ontime01 ontime %s`, c.opt.BatchMode)
			case "insert":
				command = fmt.Sprintf(`/opt/scripts/ontime_batch_insert.sh latencytest ontime01 ontime %d`, (*(c.opt)).BatchSize)
			case "dumpling":
				command = `/usr/local/bin/dumpling_data`
			}

			if err := c.workstation.RunSerialCmds([]string{command}, true); err != nil {
				return err
			}

			idx = idx + 1
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
	pexecutor          *ctxt.Executor
	opt                *operator.LatencyWhenBatchOptions
	sysbenchConfigFile string

	sysbenchResult *[][]string
	cancelCtx      *context.CancelFunc
}

// Execute implements the Task interface
func (c *RunSysbench) Execute(ctx context.Context) error {
	startTime := time.Now()

	stdout, _, err := (*c.pexecutor).Execute(context.Background(), fmt.Sprintf(`sysbench --config-file=%s %s --tables=%d --table-size=%d run`, c.sysbenchConfigFile, (*c.opt).SysbenchPluginName, (*c.opt).SysbenchNumTables, (*c.opt).SysbenchNumRows), false, 5*time.Hour)
	endTime := time.Now()

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
				// arrData = append([]string{fmt.Sprintf("%d", c.opt.BatchSize)}, arrData...)
				arrData = append(arrData, startTime.Format("15:04:05"))
				arrData = append(arrData, endTime.Format("15:04:05"))
				*c.sysbenchResult = append(*c.sysbenchResult, arrData)

			} else {
				// Todo : make compatible for two different cases
				arrData = append([]string{"Test Case", "Rows Inserted"}, arrData[:len(arrData)-1]...)
				arrData = append(arrData, "Start Time")
				arrData = append(arrData, "End Time")
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
	return fmt.Sprintf("Echo: Running tpcc")
}
