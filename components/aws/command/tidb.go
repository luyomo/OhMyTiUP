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
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/luyomo/tisample/pkg/aws/manager"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	awsutils "github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newTiDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tidb <sub_command>",
		Short: "Run commands for tidb",
	}

	cmd.AddCommand(
		newTiDBDeploy(),
		newListTiDBCmd(),
		newDestroyTiDBCmd(),
		newTiDBScale(),
		newTiDBPerfCmd(),
		newInstallThanos(),
	)
	return cmd
}

func newTiDBDeploy() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <topology.yaml>",
		Short:        "Deploy an TiDB Cluster on aws",
		Long:         "Deploy an TiDB Cluster for demo. SSH connection will be used to deploy files, as well as creating system users for running the service.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 2)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]
			topoFile := args[1]
			if data, err := os.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}

			return cm.TiDBDeploy(clusterName, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newListTiDBCmd() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "list <cluster-name>",
		Short: "List all clusters or cluster of aurora db",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.ListTiDBCluster(clusterName, opt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newDestroyTiDBCmd() *cobra.Command {
	destroyOpt := operator.Options{}
	cmd := &cobra.Command{
		Use:   "destroy <cluster-name>",
		Short: "Destroy a specified cluster",
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
			if len(destroyOpt.RetainDataRoles) > 0 {
				validRoles := set.NewStringSet(spec.AllComponentNames()...)
				for _, role := range destroyOpt.RetainDataRoles {
					if !validRoles.Exist(role) {
						return perrs.Errorf("role name `%s` invalid", role)
					}
				}
			}

			return cm.DestroyTiDBCluster(clusterName, gOpt, destroyOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes or hosts whose data will be retained")
	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&destroyOpt.Force, "force", false, "Force will ignore remote error while destroy the cluster")

	return cmd
}

func newTiDBScale() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "scale <cluster-name> <topology.yaml>",
		Short:        "scale tidb cluster",
		Long:         "scale-in or scale-out the tidb cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 2)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]
			topoFile := args[1]
			if data, err := os.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}
			fmt.Printf("The command here is %v \n", teleCommand)
			fmt.Printf("The cluster name is <%s> \n", clusterName)

			return cm.TiDBScale(clusterName, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newTiDBPerfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perf <sub_command>",
		Short: "Run measure latency against tidb",
	}

	cmd.AddCommand(
		newTiDBLatencyMeasurementCmd(),
		newTiDBPerfRecursiveCmd(),
	)
	return cmd
}

// -- latency measurement
func newTiDBLatencyMeasurementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-isolation <sub_command>",
		Short: "Run measure latency against tidb: placement rule for resource isolation test",
	}

	cmd.AddCommand(
		newTiDBLatencyMeasurementPrepareCmd(),
		newTiDBLatencyMeasurementRunCmd(),
		newTiDBLatencyMeasurementCleanupCmd(),
	)
	return cmd
}

func newTiDBLatencyMeasurementPrepareCmd() *cobra.Command {

	opt := operator.LatencyWhenBatchOptions{
		TiKVMode: "simple",
	}

	cmd := &cobra.Command{
		Use:   "prepare <cluster-name>",
		Short: "Prepare resource for test",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBMeasureLatencyPrepareCluster(clusterName, opt, gOpt)
		},
	}

	// One parameter to decide the test case - TiKV partition/Simple
	cmd.Flags().StringVarP(&opt.TiKVMode, "tikv-mode", "m", "simple", "simple: No partition for TiKV nodes.  partition: Group the TiKV to online/batch. Batch query to batch TiKV nodes, sysbench to online TiKV nodes")
	cmd.Flags().IntVar(&opt.SysbenchNumTables, "sysbench-num-tables", 8, "sysbench: --tables")
	cmd.Flags().IntVar(&opt.SysbenchNumRows, "sysbench-num-rows", 10000, "sysbench: --table-size")
	cmd.Flags().StringVarP(&opt.SysbenchDBName, "sysbench-db-name", "d", "sbtest", "sysbench: database-name")
	cmd.Flags().StringVarP(&opt.SysbenchPluginName, "sysbench-plugin-name", "p", "oltp_point_select", "sysbench: oltp_point_select")

	cmd.Flags().Int64Var(&opt.SysbenchExecutionTime, "sysbench-execution-time", 600, "sysbench: --execution-time")
	cmd.Flags().IntVar(&opt.SysbenchThread, "sysbench-thread", 4, "sysbench: --thread")
	cmd.Flags().IntVar(&opt.SysbenchReportInterval, "sysbench-report-interval", 10, "sysbench: --report-interval")

	return cmd
}

func newTiDBLatencyMeasurementRunCmd() *cobra.Command {

	opt := operator.LatencyWhenBatchOptions{
		TransInterval: 2,
	}

	cmd := &cobra.Command{
		Use:   "run <cluster-name>",
		Short: "Run the query for latency performance test",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBMeasureLatencyRunCluster(clusterName, opt, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.BatchSizeArray, "batch-size", "s", "x,10000", "Batch size: x,5000,10000,25000,50000 -> Loop the test as <no batch -> 5000 -> 10000 -> 25000 -> 50000>")
	cmd.Flags().IntVar(&opt.RunCount, "repeats", 1, "Count to loop the test")
	cmd.Flags().IntVar(&opt.TransInterval, "trans-interval", 2, "The interval to insert the transaction")

	return cmd
}

func newInstallThanos() *cobra.Command {
	opt := operator.ThanosS3Config{}

	cmd := &cobra.Command{
		Use:          "install-thanos <cluster-name>",
		Short:        "Install thanos on the TiDB cluster",
		Long:         "Install thanos on the TiDB cluster. SSH connection will be used to deploy files, as well as creating system users for running the service.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			// Fetch bucket region
			opt.Region, err = awsutils.GetS3BucketLocation(opt.Bucket)
			if err != nil {
				if strings.HasSuffix(err.Error(), "api error AccessDenied: Access Denied") {
					fmt.Printf("No S3 bucket <%s> found or no permission to access \n", opt.Bucket)
				}
				return err
			}

			// Fetch access key and secret key
			_crentials, err := awsutils.GetAWSCrential()
			if err != nil {
				return err
			}
			opt.AccessKey = (*_crentials).AccessKeyID
			opt.SecretKey = (*_crentials).SecretAccessKey

			return cm.InstallThanos(clusterName, opt, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.Bucket, "bucket", "b", "", "Bucket name for promethus data export")
	// cmd.Flags().StringVarP(&opt.Region, "region", "r", "", "Region for promethus data export")
	// cmd.Flags().StringVarP(&opt.AccessKey, "access-key", "k", "", "Access Key for promethus data export. Default: crentials")
	// cmd.Flags().StringVarP(&opt.SecretKey, "secret-key", "s", "", "Secret Key for promethus data export. Default: crentials")

	return cmd
}

func newTiDBLatencyMeasurementCleanupCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "cleanup <cluster-name>",
		Short: "Cleanup resource for test",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBMeasureLatencyCleanupCluster(clusterName, gOpt)
		},
	}

	return cmd
}

func newTiDBPerfRecursiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recursive <sub_command>",
		Short: "Test recursive query on TiFlash",
	}

	cmd.AddCommand(
		newTiDBPerfRecursivePrepareCmd(),
		newTiDBPerfRecursiveRunCmd(),
		newTiDBPerfRecursiveCleanupCmd(),
	)
	return cmd
}

func newTiDBPerfRecursivePrepareCmd() *cobra.Command {

	opt := operator.LatencyWhenBatchOptions{
		TiKVMode: "simple",
	}

	cmd := &cobra.Command{
		Use:   "prepare <cluster-name>",
		Short: "Prepare resource for test",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBRecursivePrepareCluster(clusterName, opt, gOpt)
		},
	}

	// One parameter to decide the test case - TiKV partition/Simple
	cmd.Flags().StringVarP(&opt.TiKVMode, "tikv-mode", "m", "simple", "simple: No partition for TiKV nodes.  partition: Group the TiKV to online/batch. Batch query to batch TiKV nodes, sysbench to online TiKV nodes")
	cmd.Flags().IntVar(&opt.SysbenchNumTables, "sysbench-num-tables", 8, "sysbench: --tables")
	cmd.Flags().IntVar(&opt.SysbenchNumRows, "sysbench-num-rows", 10000, "sysbench: --table-size")
	cmd.Flags().StringVarP(&opt.SysbenchDBName, "sysbench-db-name", "d", "sbtest", "sysbench: database-name")
	cmd.Flags().StringVarP(&opt.SysbenchPluginName, "sysbench-plugin-name", "p", "oltp_point_select", "sysbench: oltp_point_select")

	cmd.Flags().Int64Var(&opt.SysbenchExecutionTime, "sysbench-execution-time", 600, "sysbench: --execution-time")
	cmd.Flags().IntVar(&opt.SysbenchThread, "sysbench-thread", 4, "sysbench: --thread")
	cmd.Flags().IntVar(&opt.SysbenchReportInterval, "sysbench-report-interval", 10, "sysbench: --report-interval")

	return cmd
}

func newTiDBPerfRecursiveRunCmd() *cobra.Command {

	var numUsers, numPayments string

	cmd := &cobra.Command{
		Use:   "run <cluster-name>",
		Short: "Run the query for recursive query on TiFlash",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBRecursiveRunCluster(clusterName, numUsers, numPayments, gOpt)
		},
	}

	cmd.Flags().StringVarP(&numUsers, "num-users", "u", "1000-3000/1000", "Number of users to test, default is 1000, 2000, 3000")
	cmd.Flags().StringVarP(&numPayments, "num-payments", "p", "10000-30000/10000", "Number of payment to test, default is 10000, 2000, 3000")

	return cmd
}

func newTiDBPerfRecursiveCleanupCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "cleanup <cluster-name>",
		Short: "Cleanup resource for test",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.TiDBPerfRecursiveCleanupCluster(clusterName, gOpt)
		},
	}

	return cmd
}
