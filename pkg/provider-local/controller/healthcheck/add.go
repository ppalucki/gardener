// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

var (
	defaultSyncPeriod = time.Second * 30
	// DefaultAddOptions are the default DefaultAddArgs for AddToManager.
	DefaultAddOptions = healthcheck.DefaultAddArgs{
		HealthCheckConfig: extensionsconfig.HealthCheckConfig{
			SyncPeriod: metav1.Duration{Duration: defaultSyncPeriod},
			// Increase default QPS and Burst by factor 10 as a configuration example of custom REST options for shoot clients
			ShootRESTOptions: &extensionsconfig.RESTOptions{
				QPS:   pointer.Float32(50),
				Burst: pointer.Int(100),
			},
		},
	}
	// GardenletManagesMCM specifies whether the machine-controller-manager is managed by gardenlet.
	GardenletManagesMCM bool
)

// RegisterHealthChecks registers health checks for each extension resource
// HealthChecks are grouped by extension (e.g worker), extension.type (e.g local) and  Health Check Type (e.g SystemComponentsHealthy)
func RegisterHealthChecks(ctx context.Context, mgr manager.Manager, opts healthcheck.DefaultAddArgs) error {
	var (
		healthChecks = []healthcheck.ConditionTypeToHealthCheck{{
			ConditionType: string(gardencorev1beta1.ShootEveryNodeReady),
			HealthCheck:   worker.NewNodesChecker(),
		}}
		conditionTypesToRemove = sets.New(gardencorev1beta1.ShootControlPlaneHealthy)
	)

	if !GardenletManagesMCM {
		healthChecks = append(healthChecks, healthcheck.ConditionTypeToHealthCheck{
			ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
			HealthCheck:   general.NewSeedDeploymentHealthChecker(local.MachineControllerManagerName),
		})
		conditionTypesToRemove = conditionTypesToRemove.Delete(gardencorev1beta1.ShootControlPlaneHealthy)
	}

	return healthcheck.DefaultRegistration(
		ctx,
		local.Type,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource),
		func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} },
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		mgr,
		opts,
		nil,
		healthChecks,
		conditionTypesToRemove,
	)
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return RegisterHealthChecks(ctx, mgr, DefaultAddOptions)
}
