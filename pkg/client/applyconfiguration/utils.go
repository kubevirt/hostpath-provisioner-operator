/*
Copyright 2020 The hostpath provisioner operator Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package applyconfiguration

import (
	schema "k8s.io/apimachinery/pkg/runtime/schema"

	v1beta1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	hostpathprovisionerv1beta1 "kubevirt.io/hostpath-provisioner-operator/pkg/client/applyconfiguration/hostpathprovisioner/v1beta1"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=hostpathprovisioner.kubevirt.io, Version=v1beta1
	case v1beta1.SchemeGroupVersion.WithKind("ClaimStatus"):
		return &hostpathprovisionerv1beta1.ClaimStatusApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("HostPathProvisioner"):
		return &hostpathprovisionerv1beta1.HostPathProvisionerApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("HostPathProvisionerSpec"):
		return &hostpathprovisionerv1beta1.HostPathProvisionerSpecApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("HostPathProvisionerStatus"):
		return &hostpathprovisionerv1beta1.HostPathProvisionerStatusApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("NodePlacement"):
		return &hostpathprovisionerv1beta1.NodePlacementApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("PathConfig"):
		return &hostpathprovisionerv1beta1.PathConfigApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("StoragePool"):
		return &hostpathprovisionerv1beta1.StoragePoolApplyConfiguration{}
	case v1beta1.SchemeGroupVersion.WithKind("StoragePoolStatus"):
		return &hostpathprovisionerv1beta1.StoragePoolStatusApplyConfiguration{}

	}
	return nil
}
