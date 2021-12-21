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
	//	"github.com/luyomo/tisample/pkg/logger/log"
	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
)

func (m *Manager) SysbenchTiCDC(name string, gOpt operator.Options) error {
	_, err := m.meta(name)

	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "tisample-tidb2ms"

	clusterTable := [][]string{
		// Header
		{"Execution Time", "QPS", "# of records", "Latency"},
	}

	t := task.NewBuilder().
		SysbenchTiCDC(utils.CurrentUser(), "127.0.0.1", gOpt.IdentityFile, name, clusterType, &clusterTable).
		BuildAsStep(fmt.Sprintf("  - Sysbench ticdc %s ", name))

	if err := t.Execute(ctxt.New(context.Background(), 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	tui.PrintTable(clusterTable, true)

	return nil
}

func (m *Manager) PrepareSysbenchTiCDC(name string, gOpt operator.Options, scriptParam task.ScriptParam) error {
	_, err := m.meta(name)

	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	clusterType := "tisample-tidb2ms"

	t := task.NewBuilder().
		PrepareSysbenchTiCDC(utils.CurrentUser(), "127.0.0.1", gOpt.IdentityFile, name, clusterType, scriptParam).
		BuildAsStep(fmt.Sprintf("  - Sysbench ticdc %s ", name))

	if err := t.Execute(ctxt.New(context.Background(), 1)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	return nil
}
