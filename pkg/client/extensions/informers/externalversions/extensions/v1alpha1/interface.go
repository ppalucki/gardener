// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/gardener/gardener/pkg/client/extensions/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Clusters returns a ClusterInformer.
	Clusters() ClusterInformer
	// ControlPlanes returns a ControlPlaneInformer.
	ControlPlanes() ControlPlaneInformer
	// Extensions returns a ExtensionInformer.
	Extensions() ExtensionInformer
	// Infrastructures returns a InfrastructureInformer.
	Infrastructures() InfrastructureInformer
	// Networks returns a NetworkInformer.
	Networks() NetworkInformer
	// OperatingSystemConfigs returns a OperatingSystemConfigInformer.
	OperatingSystemConfigs() OperatingSystemConfigInformer
	// Workers returns a WorkerInformer.
	Workers() WorkerInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Clusters returns a ClusterInformer.
func (v *version) Clusters() ClusterInformer {
	return &clusterInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// ControlPlanes returns a ControlPlaneInformer.
func (v *version) ControlPlanes() ControlPlaneInformer {
	return &controlPlaneInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Extensions returns a ExtensionInformer.
func (v *version) Extensions() ExtensionInformer {
	return &extensionInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Infrastructures returns a InfrastructureInformer.
func (v *version) Infrastructures() InfrastructureInformer {
	return &infrastructureInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Networks returns a NetworkInformer.
func (v *version) Networks() NetworkInformer {
	return &networkInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// OperatingSystemConfigs returns a OperatingSystemConfigInformer.
func (v *version) OperatingSystemConfigs() OperatingSystemConfigInformer {
	return &operatingSystemConfigInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Workers returns a WorkerInformer.
func (v *version) Workers() WorkerInformer {
	return &workerInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
