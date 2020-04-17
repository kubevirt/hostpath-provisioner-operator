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

package v1alpha1

import (
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostPathProvisionerSpec defines the desired state of HostPathProvisioner
// +k8s:openapi-gen=true
type HostPathProvisionerSpec struct {
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" valid:"required"`
	// PathConfig describes the location and layout of PV storage on nodes
	PathConfig PathConfig `json:"pathConfig" valid:"required"`
}

// HostPathProvisionerStatus defines the observed state of HostPathProvisioner
// +k8s:openapi-gen=true
type HostPathProvisionerStatus struct {
	// Conditions contains the current conditions observed by the operator
	Conditions []conditions.Condition `json:"conditions,omitempty" optional:"true"`
	// OperatorVersion The version of the HostPathProvisioner Operator
	OperatorVersion string `json:"operatorVersion,omitempty" optional:"true"`
	// TargetVersion The targeted version of the HostPathProvisioner deployment
	TargetVersion string `json:"targetVersion,omitempty" optional:"true"`
	// ObservedVersion The observed version of the HostPathProvisioner deployment
	ObservedVersion string `json:"observedVersion,omitempty" optional:"true"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostPathProvisioner is the Schema for the hostpathprovisioners API
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=hostpathprovisioners,scope=Cluster
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

// PathConfig contains the information needed to build the path where the PVs will be created.
// +k8s:openapi-gen=true
type PathConfig struct {
	// Path The path the directories for the PVs are created under
	Path string `json:"path,omitempty" valid:"required"`
	// UseNamingPrefix Use the name of the PVC requesting the PV as part of the directory created
	UseNamingPrefix string `json:"useNamingPrefix,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HostPathProvisioner{}, &HostPathProvisionerList{})
}
