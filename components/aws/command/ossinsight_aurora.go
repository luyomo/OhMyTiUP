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
	// "os"
	"path"
	"strconv"

	"github.com/luyomo/tisample/pkg/aws/manager"
	operator "github.com/luyomo/tisample/pkg/aws/operation"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/set"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newOssInsightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ossinsight <sub_command>",
		Short: "ossinsight aurora",
	}

	cmd.AddCommand(
		newOssInsightDeploy(),
		newListOssInsightCmd(),
		newDestroyOssInsightCmd(),
		newOssInsightCountTables(),
		newOssInsightGithubEventsVolumeAdjust(),
		newOssInsightTestCase(),
	)
	return cmd
}

// Input: private key file
//        cluster name      -> If does not exist, give error message
// Flow: 1. Extract cluster info(workstation public key / user)
//       2. Access aurora from workstation
//       3. Extract ddl from oss insight s3
//       4. Generate db objects from ddl file
//       5. Run command to import data into db
func newOssInsightDeploy() *cobra.Command {
	opt := manager.OssInsightDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <num-of-million>",
		Short:        "Deploy an ossinsight on aurora for demo",
		Long:         "Deploy an ossinsight on aurora for demo. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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

			numOfMillions := 0
			if numOfMillions, err = strconv.Atoi(args[1]); err != nil {
				return err
			}

			return cm.OssInsightDeploy(clusterName, numOfMillions, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newOssInsightGithubEventsVolumeAdjust() *cobra.Command {
	opt := manager.OssInsightDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "adjust-github-events-volume <cluster-name> <num-of-million>",
		Short:        "Adjust the data volume to specified number",
		Long:         "Adjust the data volume to specified number. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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

			numOfMillions := 0
			if numOfMillions, err = strconv.Atoi(args[1]); err != nil {
				return err
			}

			return cm.OssInsightAdjustGithubEventsValume(clusterName, numOfMillions, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newOssInsightCountTables() *cobra.Command {
	opt := manager.OssInsightDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "count <cluster-name>",
		Short:        "Count all tables",
		Long:         "Count all the tables in the ossinsight database.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.OssInsightCountTables(clusterName, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newOssInsightTestCase() *cobra.Command {
	opt := manager.OssInsightDeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "test <cluster-name> <number-execution> <query-pattern>",
		Short: "performance test",
		Long: `Example: test aurotest 10 count
query-pattern:
  - count: Count github_events
  - mouthly_rank: ... ...
  - dynamicTrends.BarChartRace: ... ...
  - dynamicTrends.HistoricalRanking ... ...
  - dynamicTrends.TopTen ... ...
`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 3)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]
			numExeTime := 0
			if numExeTime, err = strconv.Atoi(args[1]); err != nil {
				return err
			}
			queryType := args[2]

			return cm.OssInsightTestCase(clusterName, numExeTime, queryType, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	return cmd
}

func newListOssInsightCmd() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "list <cluster-name>",
		Short: "List all clusters or cluster of aurora db",
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 1)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]

			return cm.ListAuroraCluster(clusterName, opt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")

	//	fmt.Printf("The option is <%#v> \n", opt)
	return cmd
}

func newDestroyOssInsightCmd() *cobra.Command {
	destroyOpt := operator.Options{}
	cmd := &cobra.Command{
		Use:   "destroy <cluster-name>",
		Short: "Destroy a specified cluster",
		Long: `Destroy a specified cluster, which will clean the deployment binaries and data.
You can retain some nodes and roles data when destroy cluster, eg:

  $ tiup cluster destroy <cluster-name> --retain-role-data prometheus
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.11:9000
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.12`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			// Validate the retained roles to prevent unexpected deleting data
			if len(destroyOpt.RetainDataRoles) > 0 {
				validRoles := set.NewStringSet(spec.AllComponentNames()...)
				for _, role := range destroyOpt.RetainDataRoles {
					if !validRoles.Exist(role) {
						return perrs.Errorf("role name `%s` invalid", role)
					}
				}
			}

			return cm.DestroyAuroraCluster(clusterName, gOpt, destroyOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes or hosts whose data will be retained")
	cmd.Flags().StringArrayVar(&destroyOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&destroyOpt.Force, "force", false, "Force will ignore remote error while destroy the cluster")

	return cmd
}
