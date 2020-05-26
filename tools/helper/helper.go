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

package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var operatorLabels = map[string]string{
	"operator.hostpath-provisioner.kubevirt.io": "",
}

//WithOperatorLabels aggregates common lables
func WithOperatorLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	for k, v := range operatorLabels {
		_, ok := labels[k]
		if !ok {
			labels[k] = v
		}
	}

	return labels
}

//CreateOperatorDeploymentSpec creates deployment
func CreateOperatorDeploymentSpec(name, namespace, matchKey, matchValue, serviceAccount string, numReplicas int32) *appsv1.DeploymentSpec {
	matchMap := map[string]string{matchKey: matchValue}
	spec := &appsv1.DeploymentSpec{
		Replicas: &numReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: WithOperatorLabels(matchMap),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: WithOperatorLabels(matchMap),
			},
		},
	}

	if serviceAccount != "" {
		spec.Template.Spec.ServiceAccountName = serviceAccount
	}

	return spec
}

//CreateOperatorDeployment creates deployment
func CreateOperatorDeployment(name, namespace, matchKey, matchValue, serviceAccount string, numReplicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *CreateOperatorDeploymentSpec(name, namespace, matchKey, matchValue, serviceAccount, numReplicas),
	}
	if serviceAccount != "" {
		deployment.Spec.Template.Spec.ServiceAccountName = serviceAccount
	}
	return deployment
}

//CreateOperatorContainer creates container spec for the operator pod.
func CreateOperatorContainer(name, image, verbosity string, pullPolicy corev1.PullPolicy) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: pullPolicy,
	}
}

// CreateOperatorEnvVar creates the operator container environment variables based on the passed in parameters
func CreateOperatorEnvVar(repo, deployClusterResources, operatorImage, provisionerImage, pullPolicy string) *[]corev1.EnvVar {
	return &[]corev1.EnvVar{
		{
			Name: "WATCH_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "OPERATOR_NAME",
			Value: "hostpath-provisioner-operator",
		},
		{
			Name:  "PROVISIONER_IMAGE",
			Value: provisionerImage,
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
	}
}

// CreateCRDDef creates the hostpath provisioner CRD definition.
func CreateCRDDef() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hostpathprovisioners.hostpathprovisioner.kubevirt.io",
			Labels: map[string]string{
				"operator.hostpathprovisioner.kubevirt.io": "",
			},
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "hostpathprovisioner.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Cluster",

			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:       "HostPathProvisioner",
				ListKind:   "HostPathProvisionerList",
				Plural:     "hostpathprovisioners",
				Singular:   "hostpathprovisioner",
				ShortNames: []string{"hpp", "hpps"},
			},

			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Type:        "object",
					Description: "Represents a HostPathProvisioner deployment",
					Properties: map[string]extv1beta1.JSONSchemaProps{
						"apiVersion": {
							Type: "string",
						},
						"kind": {
							Type: "string",
						},
						"metadata": {
							Type: "object",
						},
						"spec": {
							Description: "HostPathProvisionerSpec defines the desired state of HostPathProvisioner",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"imageRegistry": {
									Type: "string",
								},
								"imageTag": {
									Type: "string",
								},
								"imagePullPolicy": {
									Type: "string",
									Enum: []extv1beta1.JSON{
										{
											Raw: []byte(`"Always"`),
										},
										{
											Raw: []byte(`"IfNotPresent"`),
										},
										{
											Raw: []byte(`"Never"`),
										},
									},
								},
								"pathConfig": {
									Description: "describes the location and layout of PV storage on nodes",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"path": {
											Description: "The provisioner will store PVs at this location on each node",
											Type:        "string",
										},
										"useNamingPrefix": {
											Description: "Indicates whether the name of the requesting PVC is included in the directory name when dynamically provisioning a PV",
											Type:        "string",
										},
									},
									Type: "object",
								},
							},
							Type: "object",
						},
						"status": {
							Description: "HostPathProvisionerStatus defines the observed state of HostPathProvisioner",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"conditions": {
									Description: "Conditions contains the current conditions observed by the operator",
									Type:        "array",
								},
								"observedVersion": {
									Description: "The observed version of the HostPathProvisioner deployment",
									Type:        "string",
								},
								"operatorVersion": {
									Description: "The version of the HostPathProvisioner Operator",
									Type:        "string",
								},
								"targetVersion": {
									Description: "The targeted version of the HostPathProvisioner deployment",
									Type:        "string",
								},
							},
						},
					},
				},
			},
		},
	}
}
