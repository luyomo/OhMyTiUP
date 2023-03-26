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
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	//	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"
)

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	fmt.Printf("The context is <%#v> \n\n\n", utils.CurrentUser())
	fmt.Printf("The cluster name is %s \n\n\n", name)
	_, err := m.meta(name)
	fmt.Printf("01 Coming here %#v\n\n\n", err)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()}, []string{})
	if err != nil {
		return err
	}

	clusterType := "ohmytiup-tidb"
	//	var clusterInfo task.ClusterInfo
	t := task.NewBuilder().
		DestroyEC(&sexecutor, "test").
		DestroySecurityGroup(&sexecutor, "test").
		// DestroyVpcPeering(&sexecutor).
		DestroyNetwork(&sexecutor, "test").
		DestroyRouteTable(&sexecutor, "test").
		DestroyInternetGateway(&sexecutor, "test").
		DestroyVPC(&sexecutor, "test").
		BuildAsStep(fmt.Sprintf("  - Destroying cluster %s ", name))

	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}

// DestroyTombstone destroy and remove instances that is in tombstone state
func (m *Manager) DestroyTombstone(
	name string,
	gOpt operator.Options,
	skipConfirm bool,
) error {
	metadata, err := m.meta(name)
	// allow specific validation errors so that user can recover a broken
	// cluster if it is somehow in a bad state.
	if err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	clusterMeta := metadata.(*spec.ClusterMeta)
	cluster := clusterMeta.Topology

	if !operator.NeedCheckTombstone(cluster) {
		return nil
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	ctx := ctxt.New(context.Background(), gOpt.Concurrency)
	nodes, err := operator.DestroyTombstone(ctx, cluster, true /* returnNodesOnly */, gOpt, tlsCfg)
	if err != nil {
		return err
	}
	regenConfigTasks, _ := buildRegenConfigTasks(m, name, topo, base, nodes, true)

	t := b.
		Func("FindTomestoneNodes", func(ctx context.Context) (err error) {
			if !skipConfirm {
				err = tui.PromptForConfirmOrAbortError(
					color.HiYellowString(fmt.Sprintf("Will destroy these nodes: %v\nDo you confirm this action? [y/N]:", nodes)),
				)
				if err != nil {
					return err
				}
			}
			log.Infof("Start destroy Tombstone nodes: %v ...", nodes)
			return err
		}).
		ClusterOperate(cluster, operator.DestroyTombstoneOperation, gOpt, tlsCfg).
		UpdateMeta(name, clusterMeta, nodes).
		UpdateTopology(name, m.specManager.Path(name), clusterMeta, nodes).
		ParallelStep("+ Refresh instance configs", true, regenConfigTasks...).
		Parallel(true, buildReloadPromTasks(metadata.GetTopology())...).
		Build()

	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Destroy success")

	return nil
}
