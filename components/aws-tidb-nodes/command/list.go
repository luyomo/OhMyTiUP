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
    "path"
    "fmt"

	"github.com/spf13/cobra"
    "github.com/luyomo/tisample/pkg/aws-tidb-nodes/manager"
    "github.com/luyomo/tisample/pkg/utils"
)

func newListCmd() *cobra.Command {
    opt := manager.DeployOptions{
        IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
    }
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cm.ListCluster(opt)
		},
	}

    cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

    fmt.Printf("The option is <%#v> \n", opt)
	return cmd
}
