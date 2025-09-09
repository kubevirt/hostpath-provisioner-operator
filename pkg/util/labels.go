package util

import (
	os "os"
)

const (
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
)

// GetRecommendedLabels define the labels for prometheus resources
func GetRecommendedLabels() map[string]string {
	labels := map[string]string{
		"k8s-app":                   MultiPurposeHostPathProvisionerName,
		AppKubernetesManagedByLabel: "hostpath-provisioner-operator",
		AppKubernetesComponentLabel: "storage",
	}

	// Populate installer labels from env vars
	partOfLabelVal := os.Getenv(PartOfLabelEnvVarName)
	if partOfLabelVal != "" {
		labels[AppKubernetesPartOfLabel] = partOfLabelVal
	}
	versionLabelVal := os.Getenv(VersionLabelEnvVarName)
	if versionLabelVal != "" {
		labels[AppKubernetesVersionLabel] = versionLabelVal
	}

	return labels
}
