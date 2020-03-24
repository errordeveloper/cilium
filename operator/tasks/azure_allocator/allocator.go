// Copyright 2020 Authors of Cilium
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

package azureallocator

import (
	"context"
	"fmt"
	"time"

	operatorMetrics "github.com/cilium/cilium/operator/metrics"
	operatorNodeManager "github.com/cilium/cilium/operator/tasks/node_manager"
	apiMetrics "github.com/cilium/cilium/pkg/api/metrics"
	azureAPI "github.com/cilium/cilium/pkg/azure/api"
	azureIPAM "github.com/cilium/cilium/pkg/azure/ipam"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/ipam"
	ipamMetrics "github.com/cilium/cilium/pkg/ipam/metrics"
	clientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/option"
)

var log = logging.DefaultLogger.WithField(logfields.LogSubsys, "cilium-operator/tasks/azure_allocator")

// StartAllocator starts the Azure IP allocator
func Start(clientSet clientset.Interface) error {
	var (
		azMetrics azureAPI.MetricsAPI
		iMetrics  ipam.MetricsAPI
	)

	log.Info("Starting Azure IP allocator...")

	if option.Config.AzureSubscriptionID == "" {
		return fmt.Errorf("Azure subscription ID not specified")
	}

	if option.Config.AzureResourceGroup == "" {
		return fmt.Errorf("Azure resource group not specified")
	}

	if option.Config.EnableMetrics {
		azMetrics = apiMetrics.NewPrometheusMetrics(operatorMetrics.Namespace, "azure", operatorMetrics.Registry)
		iMetrics = ipamMetrics.NewPrometheusMetrics(operatorMetrics.Namespace, operatorMetrics.Registry)
	} else {
		azMetrics = &apiMetrics.NoOpMetrics{}
		iMetrics = &ipamMetrics.NoOpMetrics{}
	}

	azureClient, err := azureAPI.NewClient(option.Config.AzureSubscriptionID,
		option.Config.AzureResourceGroup, azMetrics, option.Config.IPAMAPIQPSLimit, option.Config.IPAMAPIBurst)
	if err != nil {
		return fmt.Errorf("unable to create Azure client: %s", err)
	}
	instances := azureIPAM.NewInstancesManager(azureClient)

	nodeManager, err := operatorNodeManager.NewNodeManager(instances, clientSet, iMetrics, option.Config.ParallelAllocWorkers, false)
	if err != nil {
		return fmt.Errorf("unable to initialize Azure node manager: %s", err)
	}

	instances.Resync(context.TODO())

	// Start an interval based  background resync for safety, it will
	// synchronize the state regularly and resolve eventual deficit if the
	// event driven trigger fails, and also release excess IP addresses
	// if release-excess-ips is enabled
	go func() {
		time.Sleep(time.Minute)
		mngr := controller.NewManager()
		mngr.UpdateController("azure-api-refresh",
			controller.ControllerParams{
				RunInterval: time.Minute,
				DoFunc: func(ctx context.Context) error {
					syncTime := instances.Resync(ctx)
					nodeManager.Resync(ctx, syncTime)
					return nil
				},
			})
	}()

	nodeManager.StartSyncTask()

	return nil
}
