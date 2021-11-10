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

package main

import (
	"flag"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	semver "github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"kubevirt.io/hostpath-provisioner-operator/pkg/controller/hostpathprovisioner"
	"kubevirt.io/hostpath-provisioner-operator/tools/helper"
	"kubevirt.io/hostpath-provisioner-operator/tools/util"
)

var (
	csvVersion         = flag.String("csv-version", "", "")
	replacesCsvVersion = flag.String("replaces-csv-version", "", "")
	namespace          = flag.String("namespace", "", "")
	pullPolicy         = flag.String("pull-policy", "", "")

	logoBase64 = flag.String("logo-base64", "", "")
	verbosity  = flag.String("verbosity", "1", "")

	operatorImage    = flag.String("operator-image-name", hostpathprovisioner.OperatorImageDefault, "optional")
	provisionerImage = flag.String("provisioner-image-name", hostpathprovisioner.ProvisionerImageDefault, "optional")
	csiDriverImage   = flag.String("csi-driver-image-name", hostpathprovisioner.CsiProvisionerImageDefault, "optional")
	csiExternalHealthMonitorImage   = flag.String("csi-external-health-monitor-image-name", hostpathprovisioner.CsiExternalHealthMonitorControllerImageDefault, "optional")
	csiNodeDriverRegistrarImage   = flag.String("csi-node-driver-image-name", hostpathprovisioner.CsiNodeDriverRegistrationImageDefault, "optional")
	csiLivenessProbeImage   = flag.String("csi-liveness-probe-image-name", hostpathprovisioner.LivenessProbeImageDefault, "optional")
	csiExternalProvisionerImage   = flag.String("csi-external-provisioner-image-name", hostpathprovisioner.CsiSigStorageProvisionerImageDefault, "optional")
	csiSnapshotterImage   = flag.String("csi-snapshotter-image-name", hostpathprovisioner.SnapshotterImageDefault, "optional")
	
	dumpCRDs         = flag.Bool("dump-crds", false, "optional - dumps operator related crd manifests to stdout")
)

func main() {
	flag.Parse()

	data := NewClusterServiceVersionData{
		CsvVersion:         *csvVersion,
		ReplacesCsvVersion: *replacesCsvVersion,
		IconBase64:         *logoBase64,
	}
	data.OperatorArgs = helper.OperatorArgs {
		Namespace:          *namespace,
		ImagePullPolicy:    *pullPolicy,
		Verbosity:          *verbosity,

		OperatorImage:    *operatorImage,
		ProvisionerImage: *provisionerImage,
		CsiDriverImage: *csiDriverImage,
		CsiExternalHealthMonitorImage: *csiExternalHealthMonitorImage,
		CsiNodeDriverRegistrarImage: *csiNodeDriverRegistrarImage,
		CsiLivenessProbeImage: *csiLivenessProbeImage,
		CsiExternalProvisionerImage: *csiExternalProvisionerImage,
		CsiSnapshotterImage: *csiSnapshotterImage,
	}

	csv, err := createClusterServiceVersion(&data)
	if err != nil {
		panic(err)
	}
	util.MarshallObject(csv, os.Stdout)

	if *dumpCRDs {
		util.MarshallObject(helper.CreateCRDDef(), os.Stdout)
	}
}

//NewClusterServiceVersionData - Data arguments used to create hostpath provisioner's CSV manifest
type NewClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	IconBase64         string
	OperatorArgs helper.OperatorArgs
}

type csvPermissions struct {
	ServiceAccountName string              `json:"serviceAccountName"`
	Rules              []rbacv1.PolicyRule `json:"rules"`
}
type csvDeployments struct {
	Name string                `json:"name"`
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

type csvStrategySpec struct {
	Permissions        []csvPermissions `json:"permissions,omitempty"`
	ClusterPermissions []csvPermissions `json:"clusterPermissions"`
	Deployments        []csvDeployments `json:"deployments"`
}

func createOperatorDeployment(data *NewClusterServiceVersionData) *appsv1.Deployment {
	deployment := helper.CreateOperatorDeployment(&data.OperatorArgs)
	return deployment
}

func createClusterServiceVersion(data *NewClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {

	description := `
Hostpath provisioner is a local storage provisioner that uses kubernetes hostpath support to create directories on the host that map to a PV. These PVs are dynamically created when a new PVC is requested.
`

	deployment := createOperatorDeployment(data)

	clusterRules := getOperatorClusterRules()
	rules := getOperatorRules()

	sideEffectNone := admissionregistrationv1.SideEffectClassNone
	webhookPath := "/validate-hostpathprovisioner-kubevirt-io-v1beta1-hostpathprovisioner"
	failPolicy := admissionregistrationv1.Fail

	csvVersion, err := semver.New(data.CsvVersion)
	if err != nil {
		return nil, err
	}

	return &csvv1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterServiceVersion",
			APIVersion: "operators.coreos.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostpathprovisioneroperator." + data.CsvVersion,
			Namespace: data.OperatorArgs.Namespace,
			Annotations: map[string]string{

				"capabilities": "Full Lifecycle",
				"categories":   "Storage",
				"alm-examples": `
      [
        {
		  "apiVersion": "hostpathprovisioner.kubevirt.io/v1beta1",
		  "kind": "HostPathProvisioner",
		  "metadata": {
			"name": "hostpath-provisioner"
		  },
		  "spec": {
			"imagePullPolicy":"IfNotPresent",
			"pathConfig": {
			  "path": "/var/hpvolumes",
			  "useNamingPrefix": false
			}
          }
        }
      ]`,
				"description": "Creates and maintains hostpath provisioner deployments",
			},
		},

		Spec: csvv1.ClusterServiceVersionSpec{
			WebhookDefinitions: []csvv1.WebhookDescription{
				{
					AdmissionReviewVersions: []string{
						"v1beta1",
					},
					ContainerPort: int32(9443),
					TargetPort: &intstr.IntOrString{
						IntVal: int32(9443),
					},
					DeploymentName: deployment.Name,
					GenerateName: "validate-hostpath-provisioner.kubevirt.io",
					FailurePolicy: &failPolicy,
					Type: csvv1.ValidatingAdmissionWebhook,
					SideEffects: &sideEffectNone,
					WebhookPath: &webhookPath,
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Delete,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups: []string{
									"hostpathprovisioner.kubevirt.io",
								},
								APIVersions: []string{
									"v1beta1",
								},
							},
						},
					},
				},
			},
			DisplayName: "Hostpath Provisioner",
			Description: description,
			Keywords:    []string{"Hostpath Provisioner", "Storage"},
			Version:     version.OperatorVersion{
				Version: *csvVersion,
			},
			Maturity:    "beta",
			Replaces:    data.ReplacesCsvVersion,
			Maintainers: []csvv1.Maintainer{{
				Name:  "KubeVirt project",
				Email: "kubevirt-dev@googlegroups.com",
			}},
			Provider: csvv1.AppLink{
				Name: "KubeVirt/Hostpath-provisioner project",
			},
			Links: []csvv1.AppLink{
				{
					Name: "Hostpath Provisioner",
					URL:  "https://github.com/kubevirt/hostpath-provisioner/blob/main/README.md",
				},
				{
					Name: "Source Code",
					URL:  "https://github.com/kubevirt/hostpath-provisioner",
				},
			},
			Icon: []csvv1.Icon{{
				Data:      data.IconBase64,
				MediaType: "image/png",
			}},
			Labels: map[string]string{
				"alm-owner-hostpath-provisioner": "hostpath-provisioner-operator",
				"operated-by":                    "hostpath-provisioner-operator",
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"alm-owner-hostpath-provisioner": "hostpath-provisioner-operator",
					"operated-by":                    "hostpath-provisioner-operator",
				},
			},
			InstallModes: []csvv1.InstallMode{
				{
					Type:      csvv1.InstallModeTypeOwnNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeSingleNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeMultiNamespace,
					Supported: false,
				},
				{
					Type:      csvv1.InstallModeTypeAllNamespaces,
					Supported: false,
				},
			},
			InstallStrategy: csvv1.NamedInstallStrategy{
				StrategyName:    "deployment",
				StrategySpec: csvv1.StrategyDetailsDeployment{
					DeploymentSpecs: []csvv1.StrategyDeploymentSpec{
						{
							Name: "hostpath-provisioner-operator",
							Spec: deployment.Spec,
						},
					},
					Permissions: []csvv1.StrategyDeploymentPermissions{
						{
							ServiceAccountName: hostpathprovisioner.OperatorServiceAccountName,
							Rules:              *rules,
						},
					},
					ClusterPermissions: []csvv1.StrategyDeploymentPermissions{
						{
							ServiceAccountName: hostpathprovisioner.OperatorServiceAccountName,
							Rules:              *clusterRules,
						},
					},
				},
			},
			CustomResourceDefinitions: csvv1.CustomResourceDefinitions{

				Owned: []csvv1.CRDDescription{
					{
						Name:        "hostpathprovisioners.hostpathprovisioner.kubevirt.io",
						Version:     "v1beta1",
						Kind:        "HostPathProvisioner",
						DisplayName: "HostPathProvisioner deployment",
						Description: "Represents a HostPathProvisioner deployment",
					},
				},
			},
		},
	}, nil
}

func getOperatorClusterRules() *[]rbacv1.PolicyRule {
	clusterRole := rbacv1.ClusterRole{}
	err := k8syaml.NewYAMLToJSONDecoder(strings.NewReader(helper.HppOperatorClusterRole)).Decode(&clusterRole)
	if err != nil {
		panic(err)
	}
	return &clusterRole.Rules
}

func getOperatorRules() *[]rbacv1.PolicyRule {
	role := rbacv1.Role{}
	err := k8syaml.NewYAMLToJSONDecoder(strings.NewReader(helper.HppOperatorRole)).Decode(&role)
	if err != nil {
		panic(err)
	}
	return &role.Rules
}
