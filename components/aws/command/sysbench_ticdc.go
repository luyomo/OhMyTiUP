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
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/spf13/cobra"
)

func newSysbenchTiCDCCmd() *cobra.Command {
	tidbConnInfo := operator.TiDBConnInfo{}
	cmd := &cobra.Command{
		Use:   "sysbench_ticdc <cluster-name>",
		Short: "sysbench test the ticdc performance",
		Long: `Destroy a specified cluster, which will clean the deployment binaries and data.
You can retain some nodes and roles data when destroy cluster, eg:

  $ tiup cluster destroy <cluster-name> --retain-role-data prometheus
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.11:9000
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.12`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			// Validate the retained roles to prevent unexpected deleting data

			return cm.SysbenchTiCDC(clusterName, gOpt, tidbConnInfo, skipConfirm)
		},
	}

	cmd.Flags().StringVarP(&tidbConnInfo.Host, "db-host", "H", "localhost", "The host of TiDB connection")
	cmd.Flags().IntVarP(&tidbConnInfo.Port, "port", "p", 4000, "The port of TiDB connection")
	cmd.Flags().StringVarP(&tidbConnInfo.DBName, "dbname", "d", "", "The dbname of TiDB connection")
	cmd.Flags().StringVarP(&tidbConnInfo.User, "user", "u", "", "The user of TiDB connection")
	cmd.Flags().StringVarP(&tidbConnInfo.Pass, "password", "P", "", "The password of TiDB connection")

	return cmd
}
