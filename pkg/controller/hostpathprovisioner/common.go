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
	ProvisionerImageDefault = "hostpath-provisioner"
	// CsiProvisionerImageDefault is the default value of the hostpath provisioner csi container image name.
	CsiProvisionerImageDefault = "hostpath-provisioner-csi"
	// CsiExternalHealthMonitorControllerImageDefault is the default value of the sig storage csi health monitor controller side car container image name.
	CsiExternalHealthMonitorControllerImageDefault = "k8s.gcr.io/sig-storage/csi-external-health-monitor-controller:v0.3.0"
	// CsiNodeDriverRegistrationImageDefault is the default value of the sig storage csi node registration side car container image name.
	CsiNodeDriverRegistrationImageDefault = "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.2.0"
	// LivenessProbeImageDefault is the default value of the liveness probe side car container image name.
	LivenessProbeImageDefault = "k8s.gcr.io/sig-storage/livenessprobe:v2.3.0"
	// SnapshotterImageDefault is the default value of the csi snapshotter side car container image name.
	SnapshotterImageDefault = "k8s.gcr.io/sig-storage/csi-snapshotter:v4.2.1"
	// CsiSigStorageProvisionerImageDefault is the default value of the sig storage csi provisioner side car container image name.
	CsiSigStorageProvisionerImageDefault = "k8s.gcr.io/sig-storage/csi-provisioner:v2.2.1"

	operatorImageEnvVarName                        = "OPERATOR_IMAGE"
	provisionerImageEnvVarName                     = "PROVISIONER_IMAGE"
	csiProvisionerImageEnvVarName                  = "CSI_PROVISIONER_IMAGE"
	externalHealthMonitorControllerImageEnvVarName = "EXTERNAL_HEALTH_MON_IMAGE"
	nodeDriverRegistrarImageEnvVarName             = "NODE_DRIVER_REG_IMAGE"
	livenessProbeImageEnvVarName                   = "LIVENESS_PROBE_IMAGE"
	snapshotterImageEnvVarName                     = "CSI_SNAPSHOT_IMAGE"
	csiSigStorageProvisionerImageEnvVarName        = "CSI_SIG_STORAGE_PROVISIONER_IMAGE"
	verbosityEnvVarName                            = "VERBOSITY"

	// OperatorServiceAccountName is the name of Service Account used to run the operator.
	OperatorServiceAccountName = "hostpath-provisioner-operator"
	// ProvisionerServiceAccountName is the name of Service Account used to run the controller.
	ProvisionerServiceAccountName = "hostpath-provisioner-admin"
	// ProvisionerServiceAccountNameCsi is the name of Service Account used to run the csi driver.
	ProvisionerServiceAccountNameCsi = "hostpath-provisioner-admin-csi"

	healthCheckName = "hostpath-provisioner-health-check"
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

	// PrometheusLabelKey provides the label to indicate prometheus metrics are available in the pods.
	PrometheusLabelKey = "prometheus.hostpathprovisioner.kubevirt.io"
	// PrometheusLabelValue provides the label value which shouldn't be empty to avoid a prometheus WIP issue.
	PrometheusLabelValue = "true"
	// PrometheusServiceName is the name of the prometheus service created by the operator.
	PrometheusServiceName = "hpp-prometheus-metrics"

	createResourceFailed  = "CreateResourceFailed"
	createResourceSuccess = "CreateResourceSuccess"

	updateResourceFailed  = "UpdateResourceFailed"
	updateResourceSuccess = "UpdateResourceSuccess"

	createMessageFailed    = "Failed to create resource %s, %v"
	createMessageSucceeded = "Successfully created resource %T %s"

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
