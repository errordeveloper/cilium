// Copyright 2019-2020 Authors of Cilium
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

package awsallocator

import (
	"context"
	"fmt"
	"time"

	operatorMetrics "github.com/cilium/cilium/operator/metrics"
	apiMetrics "github.com/cilium/cilium/pkg/api/metrics"
	ec2shim "github.com/cilium/cilium/pkg/aws/ec2"
	"github.com/cilium/cilium/pkg/aws/eni"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/ipam"
	ipamMetrics "github.com/cilium/cilium/pkg/ipam/metrics"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/option"

	"github.com/aws/aws-sdk-go-v2/aws/ec2metadata"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/sirupsen/logrus"
)

var log = logging.DefaultLogger.WithField(logfields.LogSubsys, "cilium-operator/tasks/aws_allocator")

// Init makes appropriate calls to update ENI limits based on given options.
func Init() {
	if err := eni.UpdateLimitsFromUserDefinedMappings(option.Config.AwsInstanceLimitMapping); err != nil {
		log.WithError(err).Fatal("Parse aws-instance-limit-mapping failed")
	}
	if option.Config.UpdateEC2AdapterLimitViaAPI {
		if err := eni.UpdateLimitsFromEC2API(context.TODO()); err != nil {
			log.WithError(err).Error("Unable to update instance type to adapter limits from EC2 API")
		}
	}
}

// Start kicks of ENI allocation, the initial connection to AWS
// APIs is done in a blocking manner, given that is successful, a controller is
// started to manage allocation based on CiliumNode custom resources
func Start(nodeManager *ipam.NodeManager) error {
	log.Info("Starting ENI allocator...")

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %s", err)
	}

	log.Info("Retrieving own metadata from EC2 metadata server...")
	metadataClient := ec2metadata.New(cfg)
	instance, err := metadataClient.GetInstanceIdentityDocument()
	if err != nil {
		return fmt.Errorf("unable to retrieve instance identity document: %s", err)
	}

	log.WithFields(logrus.Fields{
		"instance": instance.InstanceID,
		"region":   instance.Region,
	}).Info("Connected to EC2 metadata server")

	cfg.Region = instance.Region

	var (
		ec2Client *ec2shim.Client
		instances *eni.InstancesManager
	)

	if option.Config.EnableMetrics {
		aMetrics := apiMetrics.NewPrometheusMetrics("ipam", operatorMetric.Namespace, registry)
		ec2Client = ec2shim.NewClient(ec2.New(cfg), aMetrics, option.Config.IPAMAPIQPSLimit, option.Config.IPAMAPIBurst)
		log.Info("Connected to EC2 service API")
		iMetrics := ipamMetrics.NewPrometheusMetrics(metricNamespace, registry)

		instances = eni.NewInstancesManager(ec2Client, option.Config.ENITags)
		nodeManager, err = ipam.NewNodeManager(instances, &ciliumNodeUpdateImplementation{}, iMetrics, option.Config.ParallelAllocWorkers,
			option.Config.AwsReleaseExcessIps)
		if err != nil {
			return fmt.Errorf("unable to initialize ENI node manager: %s", err)
		}
	} else {
		ec2Client = ec2shim.NewClient(ec2.New(cfg), &apiMetrics.NoOpMetrics{}, option.Config.IPAMAPIQPSLimit, option.Config.IPAMAPIBurst)
		log.Info("Connected to EC2 service API")
		instances = eni.NewInstancesManager(ec2Client, option.Config.ENITags)
		nodeManager, err = ipam.NewNodeManager(instances, &ciliumNodeUpdateImplementation{}, &ipamMetrics.NoOpMetrics{}, option.Config.ParallelAllocWorkers,
			option.Config.AwsReleaseExcessIps)
		if err != nil {
			return fmt.Errorf("unable to initialize ENI node manager: %s", err)
		}
	}

	// Initial blocking synchronization of all ENIs and subnets
	instances.Resync(context.TODO())

	// Start an interval based  background resync for safety, it will
	// synchronize the state regularly and resolve eventual deficit if the
	// event driven trigger fails, and also release excess IP addresses
	// if release-excess-ips is enabled
	go func() {
		time.Sleep(time.Minute)
		mngr := controller.NewManager()
		mngr.UpdateController("eni-refresh",
			controller.ControllerParams{
				RunInterval: time.Minute,
				DoFunc: func(ctx context.Context) error {
					syncTime := instances.Resync(ctx)
					nodeManager.Resync(ctx, syncTime)
					return nil
				},
			})
	}()

	return nil
}
