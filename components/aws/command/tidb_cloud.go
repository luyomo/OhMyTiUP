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
	"strconv"

	"github.com/luyomo/OhMyTiUP/pkg/aws/manager"
	//	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	//"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	//"github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"

	//perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

const (
	TiDBCLOUD = "ohmytiup-tidbcloud"
)

func newTiDBCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tidb-cloud <sub_command>",
		Short: "Run commands for tidb cloud",
	}

	cmd.AddCommand(
		newTiDBCloudDeploy(),
		newListTiDBCloudCmd(),
		newPauseTiDBCloudCmd(),
		newResumeTiDBCloudCmd(),
		newDestroyTiDBCloudCmd(),
		newTiDBCloudScale(),
	)
	return cmd
}

func newTiDBCloudDeploy() *cobra.Command {
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

			return cm.TiDBCloudDeploy(clusterName, TiDBCLOUD, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newListTiDBCloudCmd() *cobra.Command {
	var status string
	var projectID uint64

	cmd := &cobra.Command{
		Use:   "list <cluster-name>",
		Short: "List TiDB Cloud Cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			var clusterName string
			if len(args) == 1 {
				clusterName = args[0]
			} else {
				clusterName = ""
			}

			return cm.ListTiDBCloudCluster(projectID, clusterName, status)
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "ALL", "Cluster status: AVAILABLE/PAUSED/RESUMING")
	cmd.PersistentFlags().Uint64Var(&projectID, "projectID", 0, "Project ID")

	return cmd
}

func newDestroyTiDBCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <project-id> <cluster-name>",
		Short: "Destroy a specified cluster",
		Long: `Destroy a specified cluster, which will clean the deployment binaries and data.
  $ aws tidb-cloud destroy <cluster-name> --projectID=1111111111`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 2)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			projectID, err := strconv.ParseUint(args[0], 10, 64)

			clusterName := args[1]

			return cm.DestroyTiDBCloudCluster(projectID, clusterName, TiDBCLOUD)
		},
	}

	return cmd
}

func newPauseTiDBCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause <project-id> <cluster-name>",
		Short: "Pause a specified cluster",
		Long: `Pause a specified cluster, which will clean the deployment binaries and data.
  $ aws tidb-cloud destroy <cluster-name> --projectID=1111111111`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 2)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			projectID, err := strconv.ParseUint(args[0], 10, 64)

			clusterName := args[1]

			return cm.PauseTiDBCloudCluster(projectID, clusterName, TiDBCLOUD)
		},
	}

	return cmd
}

func newResumeTiDBCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <project-id> <cluster-name>",
		Short: "Resume a specified cluster",
		Long: `Resume a specified cluster, which will clean the deployment binaries and data.
  $ aws tidb-cloud resume <project-id> <cluster-name>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 2)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			projectID, err := strconv.ParseUint(args[0], 10, 64)

			clusterName := args[1]

			return cm.ResumeTiDBCloudCluster(projectID, clusterName, TiDBCLOUD)
		},
	}

	return cmd
}

func newTiDBCloudScale() *cobra.Command {
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

			return cm.TiDBScale(clusterName, TiDBCLOUD, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}
