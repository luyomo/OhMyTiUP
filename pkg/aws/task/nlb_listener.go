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

package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	nlb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	smithy "github.com/aws/smithy-go"

	// "github.com/aws/aws-sdk-go-v2/service/iam/types"

	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
// func (b *Builder) CreateNLBListener(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
// 	b.tasks = append(b.tasks, &CreateNLBListener{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 		clusterInfo:    clusterInfo,
// 	})
// 	return b
// }

func (b *Builder) CreateNLBListener(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateNLBListener{BaseNLBListener: BaseNLBListener{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypePrivate}}})
	return b
}

// func (b *Builder) ListNLBListener(pexecutor *ctxt.Executor) *Builder {
// 	b.tasks = append(b.tasks, &ListNLBListener{
// 		BaseNLBListener: BaseNLBListener{BaseTask: BaseTask{pexecutor: pexecutor}},
// 	})
// 	return b
// }

// func (b *Builder) DestroyNLBListener(pexecutor *ctxt.Executor) *Builder {
// 	b.tasks = append(b.tasks, &DestroyNLBListener{
// 		BaseNLBListener: BaseNLBListener{BaseTask: BaseTask{pexecutor: pexecutor}},
// 	})
// 	return b
// }

/******************************************************************************/

type NLBListeners struct {
	BaseResourceInfo
}

func (d *NLBListeners) ToPrintTable() *[][]string {
	tableNLBListener := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(NLBListener)
		// tableNLBListener = append(tableNLBListener, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableNLBListener
}

func (d *NLBListeners) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.Listener).ListenerArn, nil
	})
}

/******************************************************************************/
type BaseNLBListener struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *nlb.Client // Replace the example to specific service
}

func (b *BaseNLBListener) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = nlb.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &NLBListeners{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseNLBListener) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	nlbArn, err := b.GetNLBArn(ContinueIfNotExists)
	if err != nil {
		return err
	}

	if nlbArn == nil {
		return nil
	}

	resp, err := b.client.DescribeListeners(context.TODO(), &nlb.DescribeListenersInput{LoadBalancerArn: nlbArn})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "ListenerNotFound" {
				return nil
			}
		}

		return err
	}

	for _, listener := range resp.Listeners {
		b.ResourceData.Append(listener)
	}
	return nil
}

/******************************************************************************/
type CreateNLBListener struct {
	BaseNLBListener
}

// Execute implements the Task interface
func (c *CreateNLBListener) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {

		nlbArn, err := c.GetNLBArn(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}

		targetGroupArn, err := c.GetTargetGroupArn(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}

		if _, err = c.client.CreateListener(context.TODO(), &nlb.CreateListenerInput{
			LoadBalancerArn: nlbArn,
			Port:            aws.Int32(4000),
			Protocol:        types.ProtocolEnumTcp,
			DefaultActions: []types.Action{
				{
					Type:           types.ActionTypeEnumForward,
					TargetGroupArn: targetGroupArn,
				},
			},
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateNLBListener) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNLBListener) String() string {
	return fmt.Sprintf("Echo: Create NLBListener ... ...  ")
}
