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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			Spec: corev1.PodSpec{
				PriorityClassName: "openshift-user-critical",
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
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("150Mi"),
			},
		},
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
		{
			Name: "INSTALLER_PART_OF_LABEL",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['app.kubernetes.io/part-of']",
				},
			},
		},
		{
			Name: "INSTALLER_VERSION_LABEL",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['app.kubernetes.io/version']",
				},
			},
		},
	}
}

// CreateCRDDef creates the hostpath provisioner CRD definition.
func CreateCRDDef() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hostpathprovisioners.hostpathprovisioner.kubevirt.io",
			Labels: map[string]string{
				"operator.hostpathprovisioner.kubevirt.io": "",
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "hostpathprovisioner.kubevirt.io",
			Scope: "Cluster",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type:        "object",
							Description: "HostPathProvisioner is the Schema for the hostpathprovisioners API",
							Properties: map[string]extv1.JSONSchemaProps{
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
									Properties: map[string]extv1.JSONSchemaProps{
										"imageRegistry": {
											Type: "string",
										},
										"imageTag": {
											Type: "string",
										},
										"imagePullPolicy": {
											Description: "PullPolicy describes a policy for if/when to pull a container image",
											Type:        "string",
											Enum: []extv1.JSON{
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
											Description: "PathConfig describes the location and layout of PV storage on nodes",
											Properties: map[string]extv1.JSONSchemaProps{
												"path": {
													Description: "Path The path the directories for the PVs are created under",
													Type:        "string",
												},
												"useNamingPrefix": {
													Description: "UseNamingPrefix Use the name of the PVC requesting the PV as part of the directory created",
													Type:        "string",
												},
											},
											Type: "object",
										},
										"workload": {
											Description: "Restrict on which nodes CDI workload pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
																											Properties: map[string]extv1.JSONSchemaProps{
																												"key": {
																													Description: "key is the label key that the selector applies to.",
																													Type:        "string",
																												},
																												"operator": {
																													Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																													Type:        "string",
																												},
																												"values": {
																													Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																													Type:        "array",
																													Items: &extv1.JSONSchemaPropsOrArray{
																														Schema: &extv1.JSONSchemaProps{
																															Type: "string",
																														},
																													},
																												},
																											},
																											Required: []string{
																												"key",
																												"operator",
																											},
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "key is the label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
																											Properties: map[string]extv1.JSONSchemaProps{
																												"key": {
																													Description: "key is the label key that the selector applies to.",
																													Type:        "string",
																												},
																												"operator": {
																													Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																													Type:        "string",
																												},
																												"values": {
																													Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																													Type:        "array",
																													Items: &extv1.JSONSchemaPropsOrArray{
																														Schema: &extv1.JSONSchemaProps{
																															Type: "string",
																														},
																													},
																												},
																											},
																											Required: []string{
																												"key",
																												"operator",
																											},
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "key is the label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
									},
									Type: "object",
									Required: []string{
										"pathConfig",
									},
								},
								"status": {
									Description: "HostPathProvisionerStatus defines the observed state of HostPathProvisioner",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "Conditions contains the current conditions observed by the operator",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "Condition represents the state of the operator's reconciliation functionality.",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Format: "date-time",
															Type:   "string",
														},
														"lastTransitionTime": {
															Format: "date-time",
															Type:   "string",
														},
														"message": {
															Type: "string",
														},
														"reason": {
															Type: "string",
														},
														"status": {
															Type: "string",
														},
														"type": {
															Description: "ConditionType is the state of the operator's reconciliation functionality.",
															Type:        "string",
														},
													},
													Required: []string{
														"status",
														"type",
													},
													Type: "object",
												},
											},
											Type: "array",
										},
										"observedVersion": {
											Description: "ObservedVersion The observed version of the HostPathProvisioner deployment",
											Type:        "string",
										},
										"operatorVersion": {
											Description: "OperatorVersion The version of the HostPathProvisioner Operator",
											Type:        "string",
										},
										"targetVersion": {
											Description: "TargetVersion The targeted version of the HostPathProvisioner deployment",
											Type:        "string",
										},
									},
									Type: "object",
								},
							},
						},
					},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type:        "object",
							Description: "HostPathProvisioner is the Schema for the hostpathprovisioners API",
							Properties: map[string]extv1.JSONSchemaProps{
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
									Properties: map[string]extv1.JSONSchemaProps{
										"imageRegistry": {
											Type: "string",
										},
										"imageTag": {
											Type: "string",
										},
										"imagePullPolicy": {
											Description: "PullPolicy describes a policy for if/when to pull a container image",
											Type:        "string",
											Enum: []extv1.JSON{
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
											Description: "PathConfig describes the location and layout of PV storage on nodes",
											Properties: map[string]extv1.JSONSchemaProps{
												"path": {
													Description: "Path The path the directories for the PVs are created under",
													Type:        "string",
												},
												"useNamingPrefix": {
													Description: "UseNamingPrefix Use the name of the PVC requesting the PV as part of the directory created",
													Type:        "boolean",
												},
											},
											Type: "object",
										},
										"workload": {
											Description: "Restrict on which nodes CDI workload pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
																											Properties: map[string]extv1.JSONSchemaProps{
																												"key": {
																													Description: "key is the label key that the selector applies to.",
																													Type:        "string",
																												},
																												"operator": {
																													Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																													Type:        "string",
																												},
																												"values": {
																													Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																													Type:        "array",
																													Items: &extv1.JSONSchemaPropsOrArray{
																														Schema: &extv1.JSONSchemaProps{
																															Type: "string",
																														},
																													},
																												},
																											},
																											Required: []string{
																												"key",
																												"operator",
																											},
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "key is the label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
																											Properties: map[string]extv1.JSONSchemaProps{
																												"key": {
																													Description: "key is the label key that the selector applies to.",
																													Type:        "string",
																												},
																												"operator": {
																													Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																													Type:        "string",
																												},
																												"values": {
																													Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																													Type:        "array",
																													Items: &extv1.JSONSchemaPropsOrArray{
																														Schema: &extv1.JSONSchemaProps{
																															Type: "string",
																														},
																													},
																												},
																											},
																											Required: []string{
																												"key",
																												"operator",
																											},
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "key is the label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																											Type:        "array",
																											Items: &extv1.JSONSchemaPropsOrArray{
																												Schema: &extv1.JSONSchemaProps{
																													Type: "string",
																												},
																											},
																										},
																									},
																									Required: []string{
																										"key",
																										"operator",
																									},
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
									},
									Type: "object",
									Required: []string{
										"pathConfig",
									},
								},
								"status": {
									Description: "HostPathProvisionerStatus defines the observed state of HostPathProvisioner",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "Conditions contains the current conditions observed by the operator",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "Condition represents the state of the operator's reconciliation functionality.",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Format: "date-time",
															Type:   "string",
														},
														"lastTransitionTime": {
															Format: "date-time",
															Type:   "string",
														},
														"message": {
															Type: "string",
														},
														"reason": {
															Type: "string",
														},
														"status": {
															Type: "string",
														},
														"type": {
															Description: "ConditionType is the state of the operator's reconciliation functionality.",
															Type:        "string",
														},
													},
													Required: []string{
														"status",
														"type",
													},
													Type: "object",
												},
											},
											Type: "array",
										},
										"observedVersion": {
											Description: "ObservedVersion The observed version of the HostPathProvisioner deployment",
											Type:        "string",
										},
										"operatorVersion": {
											Description: "OperatorVersion The version of the HostPathProvisioner Operator",
											Type:        "string",
										},
										"targetVersion": {
											Description: "TargetVersion The targeted version of the HostPathProvisioner deployment",
											Type:        "string",
										},
									},
									Type: "object",
								},
							},
						},
					},
				},
			},
			Names: extv1.CustomResourceDefinitionNames{
				Kind:       "HostPathProvisioner",
				ListKind:   "HostPathProvisionerList",
				Plural:     "hostpathprovisioners",
				Singular:   "hostpathprovisioner",
				ShortNames: []string{"hpp", "hpps"},
			},
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
		},
	}
}
