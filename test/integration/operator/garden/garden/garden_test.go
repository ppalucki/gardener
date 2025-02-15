// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden_test

import (
	"context"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardencontroller "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/operationannotation"
	thirdpartyapiutil "github.com/gardener/gardener/third_party/controller-runtime/pkg/apiutil"
)

var _ = Describe("Garden controller tests", func() {
	var (
		loadBalancerServiceAnnotations = map[string]string{"foo": "bar"}
		garden                         *operatorv1alpha1.Garden
		testRunID                      string
		testNamespace                  *corev1.Namespace
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true))
		DeferCleanup(test.WithVars(
			&etcd.DefaultInterval, 100*time.Millisecond,
			&etcd.DefaultTimeout, 500*time.Millisecond,
			&gardeneraccess.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&istio.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&kubeapiserverexposure.DefaultInterval, 100*time.Millisecond,
			&kubeapiserverexposure.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserver.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubeapiserver.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubeapiserver.Until, untilInTest,
			&kubecontrollermanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubecontrollermanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubecontrollermanager.Until, untilInTest,
			&nginxingress.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&resourcemanager.SkipWebhookDeployment, true,
			&resourcemanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&resourcemanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&resourcemanager.Until, untilInTest,
			&shared.IntervalWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
			&managedresources.IntervalWait, 100*time.Millisecond,
		))

		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
		testRunID = testNamespace.Name

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Setup manager")
		mapper, err := thirdpartyapiutil.NewDynamicRESTMapper(restConfig)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             operatorclient.RuntimeScheme,
			MetricsBindAddress: "0",
			Cache: cache.Options{
				Mapper: mapper,
				ByObject: map[client.Object]cache.ByObject{
					&operatorv1alpha1.Garden{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			},
			ClientDisableCacheFor: []client.Object{
				&corev1.Secret{}, // applied because of operations on managed resources and their secrets
			},
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		// The controller waits for the operation annotation to be removed from Etcd resources, so we need to add a
		// reconciler for it since envtest does not run the responsible controller (etcd-druid).
		Expect((&operationannotation.Reconciler{ForObject: func() client.Object { return &druidv1alpha1.Etcd{} }}).AddToManager(mgr)).To(Succeed())

		By("Register controller")
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     "10.1.0.0/16",
						Services: "10.2.0.0/16",
					},
					Ingress: gardencorev1beta1.Ingress{
						Domain: "ingress.runtime-garden.local.gardener.cloud",
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
							Annotations: loadBalancerServiceAnnotations,
						},
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: pointer.Bool(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []string{"virtual-garden.local.gardener.cloud"},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.26.3",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: "100.64.0.0/13",
					},
				},
			},
		}

		gardenClientMap := fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), mgr.GetClient()).Build()

		Expect((&gardencontroller.Reconciler{
			Config: config.OperatorConfiguration{
				Controllers: config.ControllerConfiguration{
					Garden: config.GardenControllerConfig{
						ConcurrentSyncs: pointer.Int(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Minute},
						ETCDConfig: &gardenletconfig.ETCDConfig{
							ETCDController:      &gardenletconfig.ETCDController{Workers: pointer.Int64(5)},
							CustodianController: &gardenletconfig.CustodianController{Workers: pointer.Int64(5)},
							BackupCompactionController: &gardenletconfig.BackupCompactionController{
								EnableBackupCompaction: pointer.Bool(false),
								Workers:                pointer.Int64(5),
								EventsThreshold:        pointer.Int64(100),
							},
						},
					},
				},
				FeatureGates: map[string]bool{
					"HVPA": true,
				},
			},
			Identity:        &gardencorev1beta1.Gardener{Name: "test-gardener"},
			GardenClientMap: gardenClientMap,
			GardenNamespace: testNamespace.Name,
		}).AddToManager(mgr)).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		// When the gardener-{apiserver,admission-controller} deployments are not found, the Garden controller will
		// trigger another reconciliation to enable the SeedAuthorizer feature. Since gardener-resource-manager does not
		// run in this test to create them, let's create them manually here to prevent the controller from looping
		// endlessly. We create them before the Garden resource to prevent that the test runs into a timeout.
		By("Create gardener-{apiserver,admission-controller} deployments to prevent infinite reconciliation loops")
		gardenerAPIServerDeployment := newDeployment("gardener-apiserver", testNamespace.Name)
		gardenerAdmissionControllerDeployment := newDeployment("gardener-admission-controller", testNamespace.Name)
		Expect(testClient.Create(ctx, gardenerAPIServerDeployment)).To(Succeed())
		Expect(testClient.Create(ctx, gardenerAdmissionControllerDeployment)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, gardenerAPIServerDeployment)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, gardenerAdmissionControllerDeployment)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

			By("Forcefully remove finalizers")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, garden))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})
	})

	It("should successfully reconcile and delete a Garden", func() {
		By("Wait for Garden to have finalizer")
		Eventually(func(g Gomega) []string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Finalizers
		}).Should(ConsistOf("gardener.cloud/operator"))

		By("Wait for last operation state to be set to Progressing")
		Eventually(func(g Gomega) gardencorev1beta1.LastOperationState {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			if garden.Status.LastOperation == nil {
				return ""
			}
			return garden.Status.LastOperation.State
		}).Should(Equal(gardencorev1beta1.LastOperationStateProcessing))
		Expect(garden.Status.Gardener).NotTo(BeNil())

		By("Verify that the custom resource definitions have been created")
		Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return crdList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpas.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcds.druid.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcdcopybackupstasks.druid.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("managedresources.resources.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalers.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalercheckpoints.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("authorizationpolicies.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("destinationrules.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("envoyfilters.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gateways.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("peerauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("proxyconfigs.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("requestauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("serviceentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sidecars.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("telemetries.telemetry.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtualservices.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("wasmplugins.extensions.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadgroups.networking.istio.io")})}),
			// fluent-operator
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfilters.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfluentbitconfigs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterinputs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusteroutputs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterparsers.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbits.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("collectors.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbitconfigs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("filters.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("parsers.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("outputs.fluentbit.fluent.io")})}),
		))

		By("Verify that garden runtime CA secret was generated")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"name": "ca-garden-runtime", "managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(HaveLen(1))

		By("Verify that garden namespace was labeled and annotated appropriately")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
			g.Expect(testNamespace.Labels).To(HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"))
			g.Expect(testNamespace.Labels).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"))
			g.Expect(testNamespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"))
		}).Should(Succeed())

		// The garden controller waits for the gardener-resource-manager Deployment to be healthy, so let's fake this here.
		By("Patch gardener-resource-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Verify that the ManagedResources related to runtime components have been deployed")
		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("garden-system")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vpa")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpa")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-druid")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("nginx-ingress")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-operator")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-bit")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-operator-custom-resources-garden")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vali")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("plutono")})}),
		))

		// The garden controller waits for the Istio ManagedResources to be healthy, but Istio is not really running in
		// this test, so let's fake this here.
		By("Patch Istio ManagedResources to report healthiness")
		for _, name := range []string{"istio-system", "virtual-garden-istio"} {
			Eventually(makeManagedResourceHealthy(name, "istio-system")).Should(Succeed())
		}

		// The garden controller waits for the etcd-druid ManagedResources to be healthy, but it is not really running
		// in this test, so let's fake this here.
		By("Patch etcd-druid ManagedResources to report healthiness")
		Eventually(makeManagedResourceHealthy("etcd-druid", testNamespace.Name)).Should(Succeed())

		By("Verify that the virtual garden control plane components have been deployed")
		Eventually(func(g Gomega) []druidv1alpha1.Etcd {
			etcdList := &druidv1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return etcdList.Items
		}).Should(ConsistOf(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-main")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-events")})}),
		))

		Eventually(func(g Gomega) map[string]string {
			service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())
			return service.Annotations
		}).Should(Equal(utils.MergeStringMaps(loadBalancerServiceAnnotations, map[string]string{
			"networking.istio.io/exportTo":                                              "*",
			"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
			"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}}]`,
		})))

		// The garden controller waits for the Etcd resources to be healthy, but etcd-druid is not really running in
		// this test, so let's fake this here.
		By("Patch Etcd resources to report healthiness")
		Eventually(func(g Gomega) {
			for _, suffix := range []string{"main", "events"} {
				etcd := &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-etcd-" + suffix, Namespace: testNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).To(Succeed(), "for "+etcd.Name)

				patch := client.MergeFrom(etcd.DeepCopy())
				etcd.Status.ObservedGeneration = &etcd.Generation
				etcd.Status.Ready = pointer.Bool(true)
				g.Expect(testClient.Status().Patch(ctx, etcd, patch)).To(Succeed(), "for "+etcd.Name)
			}
		}).Should(Succeed())

		// The garden controller waits for the istio-ingress Service resource to be ready, but there is
		// no service controller or GRM running in this test which would make it ready, so let's fake this here.
		By("Create and patch istio-ingress Service resource to report readiness")
		Eventually(func(g Gomega) {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway",
					Namespace: "virtual-garden-istio-ingress",
				},
				Spec: corev1.ServiceSpec{
					Type:  corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 443}},
				},
			}
			g.Expect(testClient.Create(ctx, service)).To(Succeed())

			patch := client.MergeFrom(service.DeepCopy())
			service.Status.LoadBalancer.Ingress = append(service.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{Hostname: "localhost"})
			g.Expect(testClient.Status().Patch(ctx, service, patch)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-apiserver")})}),
		))

		// The garden controller waits for the virtual-garden-kube-apiserver Deployment to be healthy, so let's fake
		// this here.
		By("Patch virtual-garden-kube-apiserver deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			podList := &corev1.PodList{}
			g.Expect(testClient.List(ctx, podList, client.InNamespace(testNamespace.Name), client.MatchingLabels(kubeapiserver.GetLabels()))).To(Succeed())

			if desiredReplicas := int(pointer.Int32Deref(deployment.Spec.Replicas, 1)); len(podList.Items) != desiredReplicas {
				g.Expect(testClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testNamespace.Name), client.MatchingLabels(kubeapiserver.GetLabels()))).To(Succeed())
				for i := 0; i < desiredReplicas; i++ {
					g.Expect(testClient.Create(ctx, &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("virtual-garden-kube-apiserver-%d", i),
							Namespace: testNamespace.Name,
							Labels:    kubeapiserver.GetLabels(),
						},
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app"}}},
					})).To(Succeed(), fmt.Sprintf("create virtual-garden-kube-apiserver pod number %d", i))
				}
			}

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Bootstrapping virtual-garden-gardener-resource-manager")
		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-gardener-resource-manager")})}),
		))

		// The secret with the bootstrap certificate indicates that the bootstrapping of virtual-garden-gardener-resource-manager started.
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return secretList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-")})}),
		))

		// virtual-garden-gardener-resource manager usually sets the token-renew-timestamp when it reconciled the secret.
		// It is not running here, so we have to patch the secret by ourselves.
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			patch := client.MergeFrom(secret.DeepCopy())
			secret.Annotations["serviceaccount.resources.gardener.cloud/token-renew-timestamp"] = "2999-01-01T00:00:00Z"
			g.Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("shoot-core-gardener-resource-manager")})}),
		))

		// The garden controller waits for the shoot-core-gardener-resource-manager ManagedResource to be healthy, but virtual-garden-gardener-resource-manager is not really running in
		// this test, so let's fake this here.
		By("Patch shoot-core-gardener-resource-manager ManagedResource to report healthiness")
		Eventually(makeManagedResourceHealthy("shoot-core-gardener-resource-manager", testNamespace.Name)).Should(Succeed())

		// The secret with the bootstrap certificate should be gone when virtual-garden-gardener-resource-manager was bootstrapped.
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return secretList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-")})}),
		))

		// The garden controller waits for the virtual-garden-gardener-resource-manager Deployment to be healthy, so let's fake this here.
		By("Patch virtual-garden-gardener-resource-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			// Don't patch bootstrapping deployment but wait for final deployment
			g.Expect(deployment.Spec.Template.Spec.Volumes).ShouldNot(ContainElements(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("kubeconfig-bootstrap")}),
			))

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Patch gardener-internal kubeconfig secret to add the token usually added by virtual-garden-gardener-resource-manager")
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-internal", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			kubeconfigRaw, ok := secret.Data["kubeconfig"]
			g.Expect(ok).To(BeTrue())

			existingKubeconfig := &clientcmdv1.Config{}
			_, _, err := clientcmdlatest.Codec.Decode(kubeconfigRaw, nil, existingKubeconfig)
			g.Expect(err).NotTo(HaveOccurred())

			existingKubeconfig.AuthInfos[0].AuthInfo.Token = "foobar"

			kubeconfigRaw, err = runtime.Encode(clientcmdlatest.Codec, existingKubeconfig)
			g.Expect(err).NotTo(HaveOccurred())

			patch := client.MergeFrom(secret.DeepCopy())
			secret.Data["kubeconfig"] = kubeconfigRaw
			g.Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())
		}).Should(Succeed())

		// The garden controller waits for the shoot-core-gardeneraccess ManagedResource to be healthy, but virtual-garden-gardener-resource-manager is not really running in
		// this test, so let's fake this here.
		By("Patch shoot-core-gardeneraccess ManagedResource to report healthiness")
		Eventually(func(g Gomega) {
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "shoot-core-gardeneraccess", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())

			patch := client.MergeFrom(mr.DeepCopy())
			mr.Status.ObservedGeneration = mr.Generation
			mr.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               "ResourcesHealthy",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
				{
					Type:               "ResourcesApplied",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
			}
			g.Expect(testClient.Status().Patch(ctx, mr, patch)).To(Succeed())
		}).Should(Succeed())

		By("Ensure virtual-garden-kube-controller-manager was deployed")
		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-controller-manager")})}),
		))

		// The garden controller waits for the virtual-garden-kube-controller-manager Deployment to be healthy, so let's fake this here.
		By("Patch virtual-garden-kube-controller-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-controller-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			podList := &corev1.PodList{}
			g.Expect(testClient.List(ctx, podList, client.InNamespace(testNamespace.Name), client.MatchingLabels(map[string]string{"app": "kubernetes", "role": "controller-manager"}))).To(Succeed())

			if desiredReplicas := int(pointer.Int32Deref(deployment.Spec.Replicas, 1)); len(podList.Items) != desiredReplicas {
				g.Expect(testClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testNamespace.Name), client.MatchingLabels(map[string]string{"app": "kubernetes", "role": "controller-manager"}))).To(Succeed())
				for i := 0; i < desiredReplicas; i++ {
					g.Expect(testClient.Create(ctx, &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("virtual-garden-kube-controller-manager-%d", i),
							Namespace: testNamespace.Name,
							Labels:    map[string]string{"app": "kubernetes", "role": "controller-manager"},
						},
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app"}}},
					})).To(Succeed(), fmt.Sprintf("create virtual-garden-kube-apiserver pod number %d", i))
				}
			}

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Create gardener-apiserver Service in runtime cluster")
		// The garden controller requires the existence of the `gardener-apiserver` Service in the runtime cluster (in
		// reality, this is created asynchronously by gardener-resource-manager which is not running in this test).
		// Hence, let's manually create it to satisfy the reconciliation flow.
		gardenerAPIServerService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: testNamespace.Name,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Ports:    []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromInt(443)}},
				Selector: map[string]string{"foo": "bar"},
			},
		}
		Expect(testClient.Create(ctx, gardenerAPIServerService)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, gardenerAPIServerService)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Verify that the ManagedResources related to Gardener control plane components have been deployed")
		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-apiserver-runtime")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-apiserver-virtual")})}),
		))

		// The garden controller waits for the Gardener-related ManagedResources to be healthy, but no
		// gardener-resource-manager is running in this test, so let's fake this here.
		By("Patch Gardener-related ManagedResources to report healthiness")
		for _, name := range []string{"apiserver", "admission-controller", "controller-manager", "scheduler"} {
			Eventually(makeManagedResourceHealthy("gardener-"+name+"-runtime", testNamespace.Name)).Should(Succeed())
			Eventually(makeManagedResourceHealthy("gardener-"+name+"-virtual", testNamespace.Name)).Should(Succeed())
		}

		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-admission-controller-runtime")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-admission-controller-virtual")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-controller-manager-runtime")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-controller-manager-virtual")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-scheduler-runtime")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-scheduler-virtual")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("garden-system-virtual")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-state-metrics")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-metrics-exporter-runtime")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardener-metrics-exporter-virtual")})}),
		))

		By("Wait for last operation state to be set to Succeeded")
		Eventually(func(g Gomega) gardencorev1beta1.LastOperationState {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			if garden.Status.LastOperation == nil {
				return ""
			}
			return garden.Status.LastOperation.State
		}).Should(Equal(gardencorev1beta1.LastOperationStateSucceeded))

		By("Delete Garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())

		By("Verify that the virtual garden control plane components have been deleted")
		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-apiserver")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-controller-manager")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-gardener-resource-manager")})}),
		))

		Eventually(func(g Gomega) []druidv1alpha1.Etcd {
			etcdList := &druidv1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return etcdList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-main")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-events")})}),
		))

		By("Verify that the garden system components have been deleted")
		// When the controller succeeds then it deletes the `ManagedResource` CRD, so we only need to ensure here that
		// the `ManagedResource` API is no longer available.
		Eventually(func(g Gomega) error {
			return testClient.List(ctx, &resourcesv1alpha1.ManagedResourceList{}, client.InNamespace(testNamespace.Name))
		}).Should(BeNotFoundError())

		By("Verify that the custom resource definitions have been deleted")
		Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return crdList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpas.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcds.druid.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcdcopybackupstasks.druid.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("managedresources.resources.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalers.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalercheckpoints.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("authorizationpolicies.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("destinationrules.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("envoyfilters.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gateways.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("peerauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("proxyconfigs.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("requestauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("serviceentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sidecars.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("telemetries.telemetry.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtualservices.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("wasmplugins.extensions.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadgroups.networking.istio.io")})}),
			// fluent-operator
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfilters.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfluentbitconfigs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterinputs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusteroutputs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterparsers.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbits.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("collectors.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbitconfigs.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("filters.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("parsers.fluentbit.fluent.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("outputs.fluentbit.fluent.io")})}),
		))

		By("Verify that gardener-resource-manager has been deleted")
		Eventually(func(g Gomega) error {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
			return testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
		}).Should(BeNotFoundError())

		By("Verify that secrets have been deleted")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(BeEmpty())

		By("Ensure Garden is gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(BeNotFoundError())
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}

func makeManagedResourceHealthy(name, namespace string) func(Gomega) {
	return func(g Gomega) {
		mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed(), fmt.Sprintf("for %s/%s", namespace, name))

		patch := client.MergeFrom(mr.DeepCopy())
		mr.Status.ObservedGeneration = mr.Generation
		mr.Status.Conditions = []gardencorev1beta1.Condition{
			{
				Type:               "ResourcesHealthy",
				Status:             "True",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
			{
				Type:               "ResourcesApplied",
				Status:             "True",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
			{
				Type:               "ResourcesProgressing",
				Status:             "False",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
		}
		g.Expect(testClient.Status().Patch(ctx, mr, patch)).To(Succeed(), fmt.Sprintf("for %s/%s", namespace, name))
	}
}

func newDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			Replicas: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "foo-container",
						Image: "foo",
					}},
				},
			},
		},
	}
}
