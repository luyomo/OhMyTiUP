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

package command

import (
	//	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/spf13/cobra"
)

func newSysbenchTiCDCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sysbench_ticdc <cluster-name>",
		Short: "sysbench test the ticdc performance",
		Long: `Run the sysben to test the qps and latencies 
  $ aws tidb2ms sysbench_ticdc <cluster-name> --identity-file=Key file to access the workstation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			return cm.SysbenchTiCDC(clusterName, gOpt)
		},
	}

	return cmd
}

func newSysbenchPrepareCmd() *cobra.Command {
	scriptParam := task.ScriptParam{}

	cmd := &cobra.Command{
		Use:   "sysbench_prepare <cluster-name>",
		Short: "prepare the sysbench test tables",
		Long: `Prepare sysbench test environment.
$ aws tidb2ms sysbench_prepare [cluster-name] -H [tidb host] -p [tidb port] -P [tidb password] -u [tidb user] -d [tidb db name] --identity-file=[key to connect to workstation]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			return cm.PrepareSysbenchTiCDC(clusterName, gOpt, scriptParam)
		},
	}

	cmd.Flags().StringVarP(&scriptParam.TiDBHost, "db-host", "H", "localhost", "The host of TiDB connection")
	cmd.Flags().IntVarP(&scriptParam.TiDBPort, "port", "p", 4000, "The port of TiDB connection")
	cmd.Flags().StringVarP(&scriptParam.TiDBDB, "dbname", "d", "cdc_test", "The dbname of TiDB connection")
	cmd.Flags().StringVarP(&scriptParam.TiDBUser, "user", "u", "", "The user of TiDB connection")
	cmd.Flags().StringVarP(&scriptParam.TiDBPass, "password", "P", "", "The password of TiDB connection")
	cmd.Flags().IntVarP(&scriptParam.NumTables, "num-tables", "N", 30, "Number of tables to generate")
	cmd.Flags().IntVarP(&scriptParam.Threads, "threads", "T", 3000, "Number of threads")
	cmd.Flags().IntVarP(&scriptParam.TableSize, "table-size", "S", 50000, "Table of sizes")

	return cmd
}
