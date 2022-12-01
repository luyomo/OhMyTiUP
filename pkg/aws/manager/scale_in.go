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

package manager

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
)

// ScaleIn the cluster.
func (m *Manager) ScaleIn(
	name string,
	skipConfirm bool,
	gOpt operator.Options,
	scale func(builer *task.Builder, metadata spec.Metadata, tlsCfg *tls.Config),
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	var (
		force bool     = gOpt.Force
		nodes []string = gOpt.Nodes
	)
	if !skipConfirm {
		if force {
			if err := tui.PromptForConfirmOrAbortError(
				color.HiRedString("Forcing scale in is unsafe and may result in data loss for stateful components.\n"+
					"The process is irreversible and could NOT be cancelled.\n") +
					"Only use `--force` when some of the servers are already permanently offline.\n" +
					"Are you sure to continue? [y/N]:",
			); err != nil {
				return err
			}
		}

		if err := tui.PromptForConfirmOrAbortError(
			"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
			strings.Join(nodes, ","),
			color.HiYellowString(name)); err != nil {
			return err
		}

		log.Infof("Scale-in nodes...")
	}
	log.Infof("Scaled cluster `%s` in successfully", name)

	// Regenerate configuration
	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}
	ctx := ctxt.New(context.Background(), 1)
	workstation, err := task.GetWSExecutor(sexecutor, ctx, name, "clusterType", "c.awsWSConfigs.UserName", "c.awsWSConfigs.KeyFile")
	if err != nil {
		return err
	}
	_, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster scale-in %s  -y -N %s --transfer-timeout %d`, name, gOpt.Nodes[0], gOpt.APITimeout), false, 300*time.Second)
	if err != nil {
		return err
	}
	//delete ec2
	reservations, err := task.ListClusterEc2s(ctx, sexecutor, name)
	instanceID := ""
	for _, resvs := range reservations.Reservations {
		for _, instance := range resvs.Instances {
			if strings.HasPrefix(gOpt.Nodes[0], instance.PrivateIpAddress) {
				instanceID = instance.InstanceId
				break
			}
		}
	}

	_, _, err = sexecutor.Execute(ctx, fmt.Sprintf("aws ec2 terminate-instances --instance-ids %s", instanceID), false)
	if err != nil {
		return err
	}
	log.Infof("Delete Ec2  successfully")

	return nil
}
