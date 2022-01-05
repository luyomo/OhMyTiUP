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

	//	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	//	"github.com/luyomo/tisample/pkg/aws/clusterutil"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/meta"
	//	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
)

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyTiDB2MSCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	_, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "tisample-tidb2ms"

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()})
	if err != nil {
		return err
	}

	t0 := task.NewBuilder().
		DestroyTransitGateways(&sexecutor).
		DestroyDMSService(&sexecutor, "dmsservice").
		DestroyVpcPeering(&sexecutor).
		BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", "127.0.0.1", 22))

	builder := task.NewBuilder().
		ParallelStep("+ Destroying tidb2ms solution service ... ...", false, t0)
	t := builder.Build()
	ctx := context.WithValue(context.Background(), "clusterName", name)
	ctx = context.WithValue(ctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(ctx, 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	var destroyTasks []*task.StepDisplay

	var auroraInfo, msInfo task.ClusterInfo
	t1 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "tidb").
		BuildAsStep(fmt.Sprintf("  - Destroying EC2 nodes cluster %s ", name))

	destroyTasks = append(destroyTasks, t1)

	t2 := task.NewBuilder().
		DestroyAurora(&sexecutor, "aurora", &auroraInfo).
		BuildAsStep(fmt.Sprintf("  - Destroying aurora db cluster %s ", name))

	destroyTasks = append(destroyTasks, t2)

	t3 := task.NewBuilder().
		DestroySqlServer(&sexecutor, "sqlserver", &msInfo).
		BuildAsStep(fmt.Sprintf("  - Destroying sqlserver cluster %s ", name))

	destroyTasks = append(destroyTasks, t3)

	t4 := task.NewBuilder().
		DestroyEC2Nodes(&sexecutor, "workstation").
		BuildAsStep(fmt.Sprintf("  - Destroying workstation cluster %s ", name))

	destroyTasks = append(destroyTasks, t4)

	builder = task.NewBuilder().
		ParallelStep("+ Destroying all the componets", false, destroyTasks...)

	t = builder.Build()

	tailctx := context.WithValue(context.Background(), "clusterName", name)
	tailctx = context.WithValue(tailctx, "clusterType", clusterType)
	if err := t.Execute(ctxt.New(tailctx, 5)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}
