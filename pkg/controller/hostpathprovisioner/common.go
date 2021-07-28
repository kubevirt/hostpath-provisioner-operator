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

package hostpathprovisioner

const (
	// OperatorImageDefault is the default value of the operator container image name.
	OperatorImageDefault = "hostpath-provisioner-operator"
	// ProvisionerImageDefault is the default value of the provisioner container image name.
	ProvisionerImageDefault    = "hostpath-provisioner"
	provisionerImageEnvVarName = "PROVISIONER_IMAGE"
	// OperatorServiceAccountName is the name of Service Account used to run the operator.
	OperatorServiceAccountName = "hostpath-provisioner-operator"
	// ControllerServiceAccountName is the name of Service Account used to run the controller.
	ControllerServiceAccountName = "hostpath-provisioner-admin"
	// MultiPurposeHostPathProvisionerName is the name used for the DaemonSet, ClusterRole/Binding, SCC and k8s-app label value.
	MultiPurposeHostPathProvisionerName = "hostpath-provisioner"
	// PartOfLabelEnvVarName is the environment variable name for the part-of label value
	PartOfLabelEnvVarName = "INSTALLER_PART_OF_LABEL"
	// VersionLabelEnvVarName is the environment variable name for the version label value
	VersionLabelEnvVarName = "INSTALLER_VERSION_LABEL"

	// AppKubernetesPartOfLabel is the Kubernetes recommended part-of label
	AppKubernetesPartOfLabel = "app.kubernetes.io/part-of"
	// AppKubernetesVersionLabel is the Kubernetes recommended version label
	AppKubernetesVersionLabel = "app.kubernetes.io/version"
	// AppKubernetesManagedByLabel is the Kubernetes recommended managed-by label
	AppKubernetesManagedByLabel = "app.kubernetes.io/managed-by"
	// AppKubernetesComponentLabel is the Kubernetes recommended component label
	AppKubernetesComponentLabel = "app.kubernetes.io/component"

	createResourceStart   = "CreateResourceStart"
	createResourceFailed  = "CreateResourceFailed"
	createResourceSuccess = "CreateResourceSuccess"

	deleteResourceStart   = "DeleteResourceStart"
	deleteResourceFailed  = "DeleteResourceFailed"
	deleteResourceSuccess = "DeleteResourceSuccess"

	updateResourceStart   = "UpdateResourceStart"
	updateResourceFailed  = "UpdateResourceFailed"
	updateResourceSuccess = "UpdateResourceSuccess"

	createMessageStart     = "Started creation of resource %T %s"
	createMessageFailed    = "Failed to create resource %s, %v"
	createMessageSucceeded = "Successfully created resource %T %s"

	updateMessageStart     = "Started update of resource %T %s"
	updateMessageFailed    = "Failed to update resource %s, %v"
	updateMessageSucceeded = "Successfully updated resource %T %s"

	provisionerHealthy        = "ProvisionerHealthy"
	provisionerHealthyMessage = "Provisioner Healthy"

	watchNameSpace = "WatchNameSpace"

	deployStarted        = "DeployStarted"
	deployStartedMessage = "Started Deployment"

	upgradeStarted = "UpgradeStarted"

	reconcileFailed = "Reconcile Failed"
)
