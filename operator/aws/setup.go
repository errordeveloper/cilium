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

package aws

import (
	"context"

	"github.com/cilium/cilium/pkg/aws/eni"
	"github.com/cilium/cilium/pkg/option"
)

// UpdateLimits makes appropriate calls to update ENI limits based on given options.
func UpdateLimits() {
	if err := eni.UpdateLimitsFromUserDefinedMappings(option.Config.AwsInstanceLimitMapping); err != nil {
		log.WithError(err).Fatal("Parse aws-instance-limit-mapping failed")
	}
	if option.Config.UpdateEC2AdapterLimitViaAPI {
		if err := eni.UpdateLimitsFromEC2API(context.TODO()); err != nil {
			log.WithError(err).Error("Unable to update instance type to adapter limits from EC2 API")
		}
	}
}
