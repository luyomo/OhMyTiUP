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
	//	"github.com/luyomo/OhMyTiUP/pkg/aws/clusterutil"
	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/task"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"

	//"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	"github.com/luyomo/OhMyTiUP/pkg/meta"
	//	"github.com/luyomo/OhMyTiUP/pkg/tui"
	"github.com/luyomo/OhMyTiUP/pkg/utils"
	perrs "github.com/pingcap/errors"
)

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyTiDB2AuroraCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	fmt.Printf("The context is <%#v> \n\n\n", utils.CurrentUser())
	_, err := m.meta(name)
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

	fmt.Printf("The messaage here is ********************** \n\n\n")
	clusterType := "ohmytiup-tidb2aurora"

	//	var clusterInfo task.ClusterInfo
	t := task.NewBuilder().
		DestroyEC(&sexecutor, "test").
		DestroyDBInstance(&sexecutor, "test").
		DestroyDBCluster(&sexecutor, "test").
		DestroyDBParameterGroup(&sexecutor, "test").
		DestroyDBClusterParameterGroup(&sexecutor, "test").
		DestroyDBSubnetGroup(&sexecutor, "test").
		DestroySecurityGroup(&sexecutor, "test").
		// DestroyVpcPeering(&sexecutor).
		DestroyNetwork(&sexecutor, "test").
		DestroyRouteTable(&sexecutor, "test").
		DestroyInternetGateway(&sexecutor).
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
