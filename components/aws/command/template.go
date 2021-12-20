// Copyright 2021 PingCAP, Inc.
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
	"fmt"
	"path"

	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/tui"
	//	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

// TemplateOptions contains the options for print topology template.
type TemplateOptions struct {
	Full    bool // print full template
	MultiDC bool // print template for deploying to multiple data center
	Local   bool // print local template
}

// This is used to identify how many bool type options are set, so that an
// error can be throw if more than one is given.
func sumBool(b ...bool) int {
	n := 0
	for _, v := range b {
		if v {
			n++
		}
	}
	return n
}

func newTemplateCmd() *cobra.Command {
	// opt := TemplateOptions{}

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print node config template",
		Long: `Print templates for data migration from tidb to sqlserver. eg
aws tidb2ms template tidbcloud2ms-prod  -- Print production template for migration from tidb cloud to sqlserveer
aws tidb2ms tempalte tidbcloud2ms-minimum -- Print minimum config tempalte from migration from tidb cloud to sqlserver
aws tidb2ms tempalte tidb2ms-minimum -- Print minimum config tempalte from migration from tidb to sqlserver
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			fileName := args[0]

			fp := path.Join("examples", "aws", fmt.Sprintf("%s.yaml", fileName))
			tpl, err := embed.ReadExample(fp)
			if err != nil {
				return err
			}

			fmt.Println(string(tpl))
			return nil
		},
	}

	return cmd
}
