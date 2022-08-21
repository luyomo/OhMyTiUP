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
	"context"
	//	"errors"
	"fmt"

	//	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/task"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/utils"
	"github.com/spf13/cobra"
)

func newAcceptTiDBCloudPeering() *cobra.Command {
	//	tidbConnInfo := operator.TiDBConnInfo{}
	cmd := &cobra.Command{
		Use:   "accept-ticloud-peering <cluster-name> <cluster-type>",
		Short: "Accept all the vpc peerings from tidb cloud",
		Long: `Accept all the pending vpc peering from tidb cloud.
Convert the pending status to active, eg:

  $ tisample accept_ticloud_peering ticb_cloud_name `,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
			if err != nil {
				return err
			}

			ctx := context.WithValue(context.Background(), "clusterName", args[0])
			ctx = context.WithValue(ctx, "clusterType", args[1])
			t1 := task.NewBuilder().
				AcceptVPCPeering(&sexecutor, []string{}).
				BuildAsStep(fmt.Sprintf("  - Accept all the vpc peerings from tidb cloud"))

			builder := task.NewBuilder().
				ParallelStep("+ Initialize target host environments", false, t1)

			t := builder.Build()
			if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}
