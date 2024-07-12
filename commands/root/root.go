// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/koupleless/virtual-kubelet/common/mqtt"
	"github.com/koupleless/virtual-kubelet/java/controller"
	"github.com/koupleless/virtual-kubelet/java/model"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon
func NewCommand(ctx context.Context, c Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run provides a virtual kubelet interface for your kubernetes cluster.",
		Long: `run implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(ctx, c)
		},
	}

	installFlags(cmd.Flags(), &c)
	return cmd
}

func runRootCommand(ctx context.Context, c Opts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := setupTracing(ctx, c); err != nil {
		return err
	}

	clientID := uuid.New().String()

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"operatingSystem": c.OperatingSystem,
		"clientID":        clientID,
	}))

	config := model.BuildBaseRegisterControllerConfig{
		MqttConfig: &mqtt.ClientConfig{
			Broker:        c.MqttBroker,
			Port:          c.MqttPort,
			ClientID:      fmt.Sprintf("module-controller@@@%s", clientID),
			Username:      c.MqttUsername,
			Password:      c.MqttPassword,
			CAPath:        c.MqttCAPath,
			ClientCrtPath: c.MqttClientCrtPath,
			ClientKeyPath: c.MqttClientKeyPath,
			CleanSession:  true,
		},
		KubeConfigPath: c.KubeConfigPath,
	}

	registerController, err := controller.NewBaseRegisterController(&config)
	if err != nil {
		return err
	}

	if registerController == nil {
		return errors.New("register controller is nil")
	}

	registerController.Run(ctx)

	select {
	case <-ctx.Done():
	case <-registerController.Done():
	}

	return registerController.Err()
}
