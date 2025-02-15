// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Pod Utils", func() {
	var podSpec corev1.PodSpec

	BeforeEach(func() {
		podSpec = corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "init1",
				},
				{
					Name: "init2",
				},
			},
			Containers: []corev1.Container{
				{
					Name: "container1",
				},
				{
					Name: "container2",
				},
			},
		}
	})

	Describe("#VisitPodSpec", func() {
		It("should do nothing because object type is not handled", func() {
			Expect(VisitPodSpec(&corev1.Service{}, nil)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		test := func(obj runtime.Object, podSpec *corev1.PodSpec) {
			It("should visit and mutate PodSpec", Offset(1), func() {
				Expect(VisitPodSpec(obj, func(podSpec *corev1.PodSpec) {
					podSpec.RestartPolicy = corev1.RestartPolicyOnFailure
				})).To(Succeed())

				Expect(podSpec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
			})
		}

		Context("corev1.Pod", func() {
			var obj = &corev1.Pod{
				Spec: podSpec,
			}
			test(obj, &obj.Spec)
		})

		Context("appsv1.Deployment", func() {
			var obj = &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.Deployment", func() {
			var obj = &appsv1beta2.Deployment{
				Spec: appsv1beta2.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta1.Deployment", func() {
			var obj = &appsv1beta1.Deployment{
				Spec: appsv1beta1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1.StatefulSet", func() {
			var obj = &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.StatefulSet", func() {
			var obj = &appsv1beta2.StatefulSet{
				Spec: appsv1beta2.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta1.StatefulSet", func() {
			var obj = &appsv1beta1.StatefulSet{
				Spec: appsv1beta1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1.DaemonSet", func() {
			var obj = &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.DaemonSet", func() {
			var obj = &appsv1beta2.DaemonSet{
				Spec: appsv1beta2.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("batchv1.Job", func() {
			var obj = &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("batchv1.CronJob", func() {
			var obj = &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			test(obj, &obj.Spec.JobTemplate.Spec.Template.Spec)
		})

		Context("batchv1beta1.CronJob", func() {
			var obj = &batchv1beta1.CronJob{
				Spec: batchv1beta1.CronJobSpec{
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			test(obj, &obj.Spec.JobTemplate.Spec.Template.Spec)
		})
	})

	Describe("#VisitContainers", func() {
		It("should do nothing if there are no containers", func() {
			podSpec.InitContainers = nil
			podSpec.Containers = nil
			VisitContainers(&podSpec, func(container *corev1.Container) {
				Fail("called visitor")
			})
		})

		It("should visit and mutate all containers if no names are given", func() {
			VisitContainers(&podSpec, func(container *corev1.Container) {
				container.TerminationMessagePath = "visited"
			})

			for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
				Expect(container.TerminationMessagePath).To(Equal("visited"), "should have visited and mutated container %s", container.Name)
			}
		})

		It("should visit and mutate only containers with matching names", func() {
			names := sets.New(podSpec.InitContainers[0].Name, podSpec.Containers[0].Name)

			VisitContainers(&podSpec, func(container *corev1.Container) {
				container.TerminationMessagePath = "visited"
			}, names.UnsortedList()...)

			for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
				if names.Has(container.Name) {
					Expect(container.TerminationMessagePath).To(Equal("visited"), "should have visited and mutated container %s", container.Name)
				} else {
					Expect(container.TerminationMessagePath).To(BeEmpty(), "should not have visited and mutated container %s", container.Name)
				}
			}
		})
	})

	Describe("#AddVolume", func() {
		var volume corev1.Volume

		BeforeEach(func() {
			volume = corev1.Volume{
				Name: "volume",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "secret",
					},
				},
			}
		})

		It("should add the volume if there are none", func() {
			podSpec.Volumes = nil

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(volume))
		})

		It("should add the volume if it is not present yet", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Name += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(otherVolume, volume))
		})

		It("should do nothing if the volume is already present (overwrite=false)", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Secret.SecretName += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(otherVolume))
		})

		It("should overwrite the volume if it is already present (overwrite=false)", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Secret.SecretName += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), true)

			Expect(podSpec.Volumes).To(ConsistOf(volume))
		})
	})

	Describe("#AddVolumeMount", func() {
		var (
			container   corev1.Container
			volumeMount corev1.VolumeMount
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			volumeMount = corev1.VolumeMount{
				Name:      "volume",
				MountPath: "path",
			}
		})

		It("should add the volumeMount if there are none", func() {
			container.VolumeMounts = nil

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(volumeMount))
		})

		It("should add the volumeMount if it is not present yet", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.Name += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(otherVolumeMount, volumeMount))
		})

		It("should do nothing if the volumeMount is already present (overwrite=false)", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.MountPath += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(otherVolumeMount))
		})

		It("should overwrite the volumeMount if it is already present (overwrite=false)", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.MountPath += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), true)

			Expect(container.VolumeMounts).To(ConsistOf(volumeMount))
		})
	})

	Describe("#AddEnvVar", func() {
		var (
			container corev1.Container
			envVar    corev1.EnvVar
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			envVar = corev1.EnvVar{
				Name:  "env",
				Value: "var",
			}
		})

		It("should add the envVar if there are none", func() {
			container.Env = nil

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(envVar))
		})

		It("should add the envVar if it is not present yet", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Name += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(otherEnvVar, envVar))
		})

		It("should do nothing if the envVar is already present (overwrite=false)", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Value += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(otherEnvVar))
		})

		It("should overwrite the envVar if it is already present (overwrite=false)", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Value += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), true)

			Expect(container.Env).To(ConsistOf(envVar))
		})
	})

	Describe("#HasEnvVar", func() {
		var (
			container  corev1.Container
			envVarName string
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			envVarName = "env"
		})

		It("should return false if there are no env vars", func() {
			container.Env = nil

			Expect(HasEnvVar(container, envVarName)).To(BeFalse())
		})

		It("should return false if there are only other env vars", func() {
			container.Env = []corev1.EnvVar{
				{Name: "env1"},
				{Name: "env2"},
			}

			Expect(HasEnvVar(container, envVarName)).To(BeFalse())
		})

		It("should return true if it has the env var", func() {
			container.Env = []corev1.EnvVar{
				{Name: "env1"},
				{Name: "env"},
			}

			Expect(HasEnvVar(container, envVarName)).To(BeTrue())
		})
	})
})
