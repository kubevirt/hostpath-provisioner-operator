/*
Copyright 2026 The hostpath provisioner operator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hostpathprovisioner

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

// componentResourcesSelector returns the user provided resource requirements of a single container, or nil.
type componentResourcesSelector func(*hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements

var (
	provisionerResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.HostPathProvisioner
	}
	csiProvisionerResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.CSIProvisioner
	}
	nodeDriverRegistrarResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.NodeDriverRegistrar
	}
	livenessProbeResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.LivenessProbe
	}
	csiSnapshotterResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.CSISnapshotter
	}
	mounterResources componentResourcesSelector = func(r *hostpathprovisionerv1.ComponentResources) *corev1.ResourceRequirements {
		return r.Mounter
	}
)

// defaultContainerResources are the resource requirements of the provisioner and the CSI sidecar containers
// when they are not overridden in the CR.
func defaultContainerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("150Mi"),
		},
	}
}

// defaultMounterResources are the resource requirements of the storage pool mounter container
// when they are not overridden in the CR.
func defaultMounterResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		},
	}
}

// getContainerResources returns the resource requirements the CR specifies for a container, or the
// defaults if the CR does not specify any. A user provided value replaces the defaults as a whole.
func getContainerResources(cr *hostpathprovisionerv1.HostPathProvisioner, selector componentResourcesSelector, defaults corev1.ResourceRequirements) corev1.ResourceRequirements {
	if cr == nil || cr.Spec.Resources == nil {
		return defaults
	}
	override := selector(cr.Spec.Resources)
	if override == nil {
		return defaults
	}
	res := override.DeepCopy()
	res.Limits = canonicalizeResourceList(res.Limits)
	res.Requests = canonicalizeResourceList(res.Requests)
	return *res
}

// canonicalizeResourceList re-parses the quantities in their canonical form, so that the desired
// object compares equal to what the API server returns (for instance 0.5 is stored as 500m), and
// we don't issue needless updates on every reconcile.
func canonicalizeResourceList(list corev1.ResourceList) corev1.ResourceList {
	if list == nil {
		return nil
	}
	res := make(corev1.ResourceList, len(list))
	for name, quantity := range list {
		res[name] = resource.MustParse(quantity.String())
	}
	return res
}
