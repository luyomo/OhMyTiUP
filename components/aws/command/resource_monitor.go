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
	// "fmt"
	// "os"
	// "path"

	// "github.com/luyomo/tisample/pkg/aws/manager"
	// "github.com/luyomo/tisample/pkg/aws/spec"
	//	"github.com/luyomo/tisample/pkg/aws/task"
	// operator "github.com/luyomo/tisample/pkg/aws/operation"
	// "github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	// "github.com/luyomo/tisample/pkg/utils"
	// perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newResourceMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource <sub_command>",
		Short: "Resource monitor operations",
	}

	cmd.AddCommand(
		newListResourceCmd(),
	)
	return cmd
}

func newListResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list <resource name>",
		Short:        "List all the resources <EC2/RDS>",
		Long:         "List all the running resources",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			resourceName := args[0]

			return cm.ListAwsResources(resourceName)
		},
	}

	// cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}
