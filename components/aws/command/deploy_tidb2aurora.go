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
	//"context"
	"fmt"
	"os"
	"path"

	"github.com/luyomo/tisample/pkg/aws/manager"
	//	"github.com/luyomo/tisample/pkg/aws/spec"
	//	"github.com/luyomo/tisample/pkg/aws/task"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	"github.com/spf13/cobra"
)

func newTiDB2AuroraDeploy() *cobra.Command {
	opt := manager.TiDB2AuroraDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy_tidb2aurora <cluster-name> <topology.yaml>",
		Short:        "Deploy an aurora for demo",
		Long:         "Deploy an aurora for demo. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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

			return cm.TiDB2AuroraDeploy(clusterName, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}
