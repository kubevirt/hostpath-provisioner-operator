/*
Copyright 2019 The hostpath provisioner operator Authors.

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

package v1beta1

import (
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostPathProvisionerSpec defines the desired state of HostPathProvisioner
// +k8s:openapi-gen=true
type HostPathProvisionerSpec struct {
	// ImagePullPolicy is the container pull policy for the host path provisioner containers
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" valid:"required"`
	// PathConfig describes the location and layout of PV storage on nodes. Deprecated
	PathConfig *PathConfig `json:"pathConfig,omitempty" optional:"true"`
	// Restrict on which nodes HPP workload pods will be scheduled
	Workload NodePlacement `json:"workload,omitempty"`
	// FeatureGates are a list of specific enabled feature gates
	// +listType=set
	FeatureGates []string `json:"featureGates,omitempty"`
	// VolumeSources are a list of volume sources
	// +listType=atomic
	VolumeSources []VolumeSource `json:"volumeSources,omitempty" optional:"true"`
}

// HostPathProvisionerStatus defines the observed state of HostPathProvisioner
// +k8s:openapi-gen=true
type HostPathProvisionerStatus struct {
	// Conditions contains the current conditions observed by the operator
	// +listType=atomic
	Conditions []conditions.Condition `json:"conditions,omitempty" optional:"true"`
	// OperatorVersion The version of the HostPathProvisioner Operator
	OperatorVersion string `json:"operatorVersion,omitempty" optional:"true"`
	// TargetVersion The targeted version of the HostPathProvisioner deployment
	TargetVersion string `json:"targetVersion,omitempty" optional:"true"`
	// ObservedVersion The observed version of the HostPathProvisioner deployment
	ObservedVersion string `json:"observedVersion,omitempty" optional:"true"`
}

// VolumeSource defines a volume with a kind and a source
// +k8s:openapi-gen=true
type VolumeSource struct {
	// Kind specifies an identifier that is used in the storage class arguments to identify the source to use.
	Kind string `json:"kind" valid:"required"`
	// SourceStorageClass specifies which storage class to use to create PersistVolumeClaims against and the template of the claim
	SourceStorageClass *SourceStorageClass `json:"sourceStorageClass,omitempty" optional:"true"`
	// PVC specifies the persistent volume claim to use. This is useful when you want one PVC to be shared among all nodes
	PVC *corev1.PersistentVolumeClaimSpec `json:"pvc,omitempty" optional:"true"`
	// path the path to use on the host, this is a required field
	Path string `json:"path" valid:"required"`
}

// SourceStorageClass defines the storage class and PVC template to use when preparing storage.
// +k8s:openapi-gen=true
type SourceStorageClass struct {
	Name        string                            `json:"name" valid:"required"`
	PVCTemplate *corev1.PersistentVolumeClaimSpec `json:"pvcTemplate" valid:"required"`
}

// this has to be here otherwise informer-gen doesn't recognize it
// see https://github.com/kubernetes/code-generator/issues/59
// +genclient:nonNamespaced

// HostPathProvisioner is the Schema for the hostpathprovisioners API
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=hostpathprovisioners,scope=Cluster
// +kubebuilder:storageversion
type HostPathProvisioner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostPathProvisionerSpec   `json:"spec,omitempty"`
	Status HostPathProvisionerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostPathProvisionerList contains a list of HostPathProvisioner
type HostPathProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostPathProvisioner `json:"items"`
}

// PathConfig contains the information needed to build the path where the PVs will be created. Deprecated
// +k8s:openapi-gen=true
type PathConfig struct {
	// Path The path the directories for the PVs are created under
	Path string `json:"path,omitempty" valid:"required"`
	// UseNamingPrefix Use the name of the PVC requesting the PV as part of the directory created
	UseNamingPrefix bool `json:"useNamingPrefix,omitempty"`
}

// NodePlacement describes node scheduling configuration.
// +k8s:openapi-gen=true
type NodePlacement struct {
	// nodeSelector is the node selector applied to the relevant kind of pods
	// It specifies a map of key-value pairs: for the pod to be eligible to run on a node,
	// the node must have each of the indicated key-value pairs as labels
	// (it can have additional labels as well).
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector
	// +kubebuilder:validation:Optional
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// affinity enables pod affinity/anti-affinity placement expanding the types of constraints
	// that can be expressed with nodeSelector.
	// affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector
	// See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
	// +kubebuilder:validation:Optional
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations is a list of tolerations applied to the relevant kind of pods
	// See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info.
	// These are additional tolerations other than default ones.
	// +kubebuilder:validation:Optional
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}
