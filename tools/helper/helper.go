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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type OperatorArgs struct {
	Namespace       string
	ImagePullPolicy string
	Verbosity       string

	OperatorImage                 string
	ProvisionerImage              string
	CsiDriverImage                string
	CsiExternalHealthMonitorImage string
	CsiNodeDriverRegistrarImage   string
	CsiLivenessProbeImage         string
	CsiExternalProvisionerImage   string
	CsiSnapshotterImage           string
}

const (
	openshiftPriorityClassName = "openshift-user-critical"
)

func CreateOperatorDeployment(args *OperatorArgs) *appsv1.Deployment {
	deployment := appsv1.Deployment{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(HppOperatorDeployment)).Decode(&deployment)
	deployment.SetNamespace(args.Namespace)
	deployment.Spec.Selector.MatchLabels["operator.hostpath-provisioner.kubevirt.io"] = ""
	deployment.Spec.Template.Labels["operator.hostpath-provisioner.kubevirt.io"] = ""
	deployment.Spec.Template.Spec.PriorityClassName = "openshift-user-critical"
	deployment.Spec.Template.Spec.Containers[0].Image = args.OperatorImage
	deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullPolicy(args.ImagePullPolicy)
	deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "PRIORITY_CLASS",
		Value: openshiftPriorityClassName,
	})
	setEnvVariable("VERBOSITY", args.Verbosity, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("PROVISIONER_IMAGE", args.ProvisionerImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("CSI_PROVISIONER_IMAGE", args.CsiDriverImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("EXTERNAL_HEALTH_MON_IMAGE", args.CsiExternalHealthMonitorImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("NODE_DRIVER_REG_IMAGE", args.CsiNodeDriverRegistrarImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("LIVENESS_PROBE_IMAGE", args.CsiLivenessProbeImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("CSI_SIG_STORAGE_PROVISIONER_IMAGE", args.CsiExternalProvisionerImage, deployment.Spec.Template.Spec.Containers[0].Env)
	setEnvVariable("CSI_SNAPSHOT_IMAGE", args.CsiSnapshotterImage, deployment.Spec.Template.Spec.Containers[0].Env)
	return &deployment
}

func setEnvVariable(key, value string, env []corev1.EnvVar) {
	for _, envValue := range env {
		if envValue.Name == key {
			envValue.Value = value
		}
	}
}

// CreateCRDDef creates the hostpath provisioner CRD definition.
func CreateCRDDef() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(hppCRD)).Decode(&crd)
	return &crd
}
