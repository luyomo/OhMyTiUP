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
	// "fmt"
	"os"
	"path"

	"github.com/luyomo/tisample/pkg/aws/manager"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newTiDB2Kafka2MongoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tidb2kafka2mongo <sub_command>",
		Short: "Run commands for syncing the data to mongo from tidb through kafka",
	}

	cmd.AddCommand(
		newTiDB2Kafka2MongoDeploy(),
		newListTiDB2Kafka2MongoCmd(),
		newDestroyTiDB2Kafka2MongoCmd(),
		// newTiDB2Kafka2MongoPerfCmd(),
	)
	return cmd
}

func newTiDB2Kafka2MongoPerfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perf <sub_command>",
		Short: "Run measure latency against tidb",
	}

	cmd.AddCommand(
		newPerfPrepareTiDB2Kafka2Mongo(),
		newPerfTiDB2Kafka2Mongo(),
		newPerfCleanTiDB2Kafka2Mongo(),
	)
	return cmd
}

func newTiDB2Kafka2MongoDeploy() *cobra.Command {
	opt := manager.TiDB2Kafka2MongoDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}

	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <topology.yaml>",
		Short:        "Deploy an Kafka Cluster on aws",
		Long:         "Deploy an Kafka Cluster for demo. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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

			return cm.TiDB2Kafka2MongoDeploy(clusterName, "ohmytiup-tidb2kafka2mongo", topoFile, opt, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newListTiDB2Kafka2MongoCmd() *cobra.Command {
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

			return cm.ListTiDB2Kafka2MongoCluster(clusterName, "ohmytiup-tidb2kafka2mongo", opt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newDestroyTiDB2Kafka2MongoCmd() *cobra.Command {
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

			return cm.DestroyTiDB2Kafka2MongoCluster(clusterName, "ohmytiup-tidb2kafka2mongo", gOpt, destroyOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes or hosts whose data will be retained")
	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&destroyOpt.Force, "force", false, "Force will ignore remote error while destroy the cluster")

	return cmd
}

func newPerfPrepareTiDB2Kafka2Mongo() *cobra.Command {
	perfOpt := manager.KafkaPerfOpt{
		Partitions:    1,
		NumOfRecords:  100000,
		BytesOfRecord: 1024,
	}
	cmd := &cobra.Command{
		Use:          "prepare <cluster-name>",
		Short:        "perf performance test preparation",
		Long:         "Performance measurement against kafka cluster",
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

			return cm.PerfPrepareTiDB2Kafka2PG(clusterName, "ohmytiup-tidb2kafka2pg", perfOpt, gOpt)
		},
	}

	cmd.Flags().IntVar(&perfOpt.Partitions, "partitions", 16, "The partition number of the topic to be tested.")
	cmd.Flags().IntVar(&perfOpt.NumOfRecords, "num-of-records", 100000, "The number of messages to be tested")
	cmd.Flags().IntVar(&perfOpt.BytesOfRecord, "bytes-of-record", 1024, "Bytes of records to be tested")
	cmd.Flags().StringArrayVar(&perfOpt.DataTypeDtr, "data-type", nil, "Specify all the data types to be tested")

	return cmd
}

func newPerfTiDB2Kafka2Mongo() *cobra.Command {
	perfOpt := manager.KafkaPerfOpt{
		NumOfRecords: 100000,
	}
	cmd := &cobra.Command{
		Use:          "run <cluster-name>",
		Short:        "perf performance test",
		Long:         "Performance measurement against kafka cluster",
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

			return cm.PerfTiDB2Kafka2PG(clusterName, "ohmytiup-tidb2kafka2pg", perfOpt, gOpt)
		},
	}

	cmd.Flags().IntVar(&perfOpt.NumOfRecords, "num-of-records", 100000, "The number of messages to be tested")

	return cmd
}

func newPerfCleanTiDB2Kafka2Mongo() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "clean <cluster-name>",
		Short:        "clean perf performance test",
		Long:         "Performance measurement against kafka cluster",
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

			return cm.PerfCleanTiDB2Kafka2PG(clusterName, "ohmytiup-tidb2kafka2pg", gOpt)
		},
	}

	return cmd
}
