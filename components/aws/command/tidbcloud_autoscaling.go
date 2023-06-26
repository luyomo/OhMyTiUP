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
	"os"
	// "path"

	// "github.com/luyomo/OhMyTiUP/pkg/aws/manager"
	// operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	// "github.com/luyomo/OhMyTiUP/pkg/set"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
	// perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newTiDBCloudAutoscalingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tidbcloud-autoscaling <sub_command>",
		Short: "auto scale the TiDB or TiKV nodes on the TiDB Cloud",
	}

	cmd.AddCommand(
		newTiDBCloudAutoscalingDeploy(),
	)
	return cmd
}

func newTiDBCloudAutoscalingDeploy() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <topology.yaml>",
		Short:        "Deploy tidb cloud autoscaling environment",
		Long:         "Deploy tidb cloud and one workstation",
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

			return cm.Aurora2TiDBCloudDeploy(clusterName, topoFile, skipConfirm, gOpt)
		},
	}

	return cmd
}
