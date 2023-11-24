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
	//	"fmt"
	"os"
	"path"

	"github.com/luyomo/OhMyTiUP/pkg/aws/manager"
	// operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	// "github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"

	// perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newWorkstationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workstation <sub_command>",
		Short: "Run commands for workstation",
	}

	cmd.AddCommand(
		newWorkstationDeploy(),
		newListWorkstationCmd(),
		newDestroyWorkstationCmd(),
		newShowVPCPeeringWorkstationCmd(),
		newAcceptVPCPeeringWorkstationCmd(),
	)
	return cmd
}

func newWorkstationDeploy() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <topology.yaml>",
		Short:        "Deploy an workstation node on aws",
		Long:         "Deploy an workstation for demo. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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

			return cm.WorkstationDeploy(clusterName, topoFile, opt, skipConfirm, gOpt)
		},
	}

	return cmd
}

func newListWorkstationCmd() *cobra.Command {
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

			return cm.ListWorkstation(clusterName, opt)
		},
	}

	return cmd
}

func newDestroyWorkstationCmd() *cobra.Command {

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

			return cm.DestroyWorkstation(clusterName, gOpt, skipConfirm)
		},
	}

	return cmd
}

func newShowVPCPeeringWorkstationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-vpc-peering <cluster-name>",
		Short: "Show the vpc peering between Workstation and TiDB Cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.ShowVPCPeering(clusterName, "ohmytiup-workstation", []string{"workstation"})
		},
	}

	return cmd
}

func newAcceptVPCPeeringWorkstationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-vpc-peering <cluster-name> ",
		Short: "Accept the vpc peering between workstation and TiDB Cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.AcceptVPCPeering(clusterName, "ohmytiup-workstation", []string{"workstation"})
		},
	}

	return cmd
}
