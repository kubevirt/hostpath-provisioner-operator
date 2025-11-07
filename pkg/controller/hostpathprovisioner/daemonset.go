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

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	secv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/pkg/util"
)

const (
	csiSocket               = "/csi/csi.sock"
	nodeDriverRegistrarName = "node-driver-registrar"
	legacyStoragePoolName   = "legacy"
	maxMountNameLength      = 63
)

var (
	socketDirVolumeMount = corev1.VolumeMount{Name: "socket-dir", MountPath: "/csi"}
	selectorLabels       = map[string]string{
		"k8s-app": MultiPurposeHostPathProvisionerName,
	}
)

type daemonSetArgs struct {
	operatorImage            string
	provisionerImage         string
	nodeDriverRegistrarImage string
	livenessProbeImage       string
	snapshotterImage         string
	csiProvisionerImage      string
	namespace                string
	name                     string
	verbosity                int
	version                  string
}

// reconcileDaemonSet Reconciles the daemon set.
func (r *ReconcileHostPathProvisioner) reconcileDaemonSet(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Previous versions created resources with names that depend on the CR, whereas now, we have fixed names for those.
	// We will remove those and have the next loop create the resources with fixed names so we don't end up with two sets of hpp resources.
	dups, err := r.getDuplicateDaemonSet(cr.Name, namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	for _, dup := range dups {
		if err := r.deleteDaemonSet(dup.Name, namespace); err != nil {
			return reconcile.Result{}, err
		}
	}
	args := getDaemonSetArgs(reqLogger.WithName("daemonset args"), namespace, true)
	if r.isLegacy(cr) {
		// provisioner
		args.version = cr.Status.TargetVersion
		if res, err := r.reconcileDaemonSetForSa(reqLogger, createDaemonSetObject(cr, reqLogger, args), cr); err != nil {
			return res, err
		}
	} else {
		// remove legacy ds if it exists.
		if err := r.deleteDaemonSet(args.name, args.namespace); err != nil {
			return reconcile.Result{}, err
		}
	}
	// csi driver
	args = getDaemonSetArgs(reqLogger.WithName("daemonset args"), namespace, false)
	args.version = cr.Status.TargetVersion
	return r.reconcileDaemonSetForSa(reqLogger, r.createCSIDaemonSetObject(cr, reqLogger, args), cr)
}

func (r *ReconcileHostPathProvisioner) reconcileDaemonSetForSa(reqLogger logr.Logger, desired *appsv1.DaemonSet, cr *hostpathprovisionerv1.HostPathProvisioner) (reconcile.Result, error) {
	// Define a new DaemonSet object
	setLastAppliedConfiguration(desired)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this DaemonSet already exists
	found := &appsv1.DaemonSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DaemonSet", "DaemonSet.Namespace", desired.Namespace, "Daemonset.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// DaemonSet created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Cleanup daemonsets from previous versions where .spec.selector contains junk
	// We will remove those and have the next loop create them
	if !reflect.DeepEqual(found.Spec.Selector.MatchLabels, desired.Spec.Selector.MatchLabels) {
		if err := r.deleteDaemonSet(desired.Name, desired.Namespace); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, fmt.Errorf("DaemonSet with extra selector labels spotted, cleaning up and requeueing")
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()
	// Copy found status fields, so the compare won't fail on desired/scheduled/ready pods being different. Updating will ignore them anyway.
	desired = copyIgnoredFields(desired, found)

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	found.Spec = *desired.Spec.DeepCopy()

	if !reflect.DeepEqual(currentRuntimeObjCopy, found) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, found)
		// Current is different from desired, update.
		reqLogger.Info("Updating DaemonSet", "DaemonSet.Name", desired.Name)
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// DaemonSet already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: DaemonSet already exists", "DaemonSet.Namespace", found.Namespace, "Daemonset.Name", found.Name)
	return reconcile.Result{}, nil
}

func getDaemonSetArgs(reqLogger logr.Logger, namespace string, legacyProvisioner bool) *daemonSetArgs {
	res := &daemonSetArgs{}

	if legacyProvisioner {
		res.name = MultiPurposeHostPathProvisionerName
		res.provisionerImage = os.Getenv(provisionerImageEnvVarName)
		if res.provisionerImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", provisionerImageEnvVarName, ProvisionerImageDefault))
			res.provisionerImage = ProvisionerImageDefault
		}
	} else {
		res.name = fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)
		res.provisionerImage = os.Getenv(csiProvisionerImageEnvVarName)
		if res.provisionerImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", csiProvisionerImageEnvVarName, CsiProvisionerImageDefault))
			res.provisionerImage = CsiProvisionerImageDefault
		}

		res.nodeDriverRegistrarImage = os.Getenv(nodeDriverRegistrarImageEnvVarName)
		if res.nodeDriverRegistrarImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", nodeDriverRegistrarImageEnvVarName, CsiNodeDriverRegistrationImageDefault))
			res.nodeDriverRegistrarImage = CsiNodeDriverRegistrationImageDefault
		}

		res.livenessProbeImage = os.Getenv(livenessProbeImageEnvVarName)
		if res.livenessProbeImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", livenessProbeImageEnvVarName, LivenessProbeImageDefault))
			res.livenessProbeImage = LivenessProbeImageDefault
		}
		res.snapshotterImage = os.Getenv(snapshotterImageEnvVarName)
		if res.snapshotterImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", snapshotterImageEnvVarName, SnapshotterImageDefault))
			res.snapshotterImage = SnapshotterImageDefault
		}

		res.csiProvisionerImage = os.Getenv(csiSigStorageProvisionerImageEnvVarName)
		if res.csiProvisionerImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", csiSigStorageProvisionerImageEnvVarName, CsiSigStorageProvisionerImageDefault))
			res.csiProvisionerImage = CsiSigStorageProvisionerImageDefault
		}
		res.operatorImage = os.Getenv(operatorImageEnvVarName)
		if res.operatorImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", operatorImageEnvVarName, OperatorImageDefault))
			res.operatorImage = OperatorImageDefault
		}
	}
	res.namespace = namespace
	verbosity := os.Getenv(verbosityEnvVarName)
	if verbosity != "" {
		if v, err := strconv.Atoi(verbosity); err == nil {
			res.verbosity = v
		}
	} else {
		res.verbosity = 3
	}
	return res
}

func (r *ReconcileHostPathProvisioner) deleteDaemonSet(name, namespace string) error {
	// Check if this DaemonSet already exists
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if err := r.client.Delete(context.TODO(), ds); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func copyIgnoredFields(desired, current *appsv1.DaemonSet) *appsv1.DaemonSet {
	desired = copyStatusFields(desired, current)
	desired.Spec.Template.Spec.DeprecatedServiceAccount = current.Spec.Template.Spec.DeprecatedServiceAccount
	desired.Spec.Template.Spec.SchedulerName = current.Spec.Template.Spec.SchedulerName
	// Leave out spec.selector updates; this section is a minimal set that is needed to know which pods are under our governance, and is immutable
	desired.Spec.Selector = current.Spec.Selector.DeepCopy()
	return desired
}

func copyStatusFields(desired, current *appsv1.DaemonSet) *appsv1.DaemonSet {
	desired.Status = *current.Status.DeepCopy()
	return desired
}

// createDaemonSetObject returns a new DaemonSet in the same namespace as the cr
func createDaemonSetObject(cr *hostpathprovisionerv1.HostPathProvisioner, reqLogger logr.Logger, args *daemonSetArgs) *appsv1.DaemonSet {
	reqLogger.V(3).Info("CR nodeselector", "nodeselector", cr.Spec.Workload)
	volumeType := corev1.HostPathDirectoryOrCreate
	usePrefix := getUsePrefix(cr)
	path := getPath(cr)
	labels := util.GetRecommendedLabels()
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.name,
			Namespace: args.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            ProvisionerServiceAccountName,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					SecurityContext:               &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("150Mi"),
								},
							},
							Name:            MultiPurposeHostPathProvisionerName,
							Image:           args.provisionerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  "USE_NAMING_PREFIX",
									Value: strconv.FormatBool(usePrefix),
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
								{
									Name:  "PV_DIR",
									Value: path,
								},
								{
									Name: "INSTALLER_PART_OF_LABEL",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.labels['app.kubernetes.io/part-of']",
										},
									},
								},
								{
									Name: "INSTALLER_VERSION_LABEL",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.labels['app.kubernetes.io/version']",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "pv-volume",
									MountPath: path,
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "pv-volume", // Has to match VolumeMounts in containers
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: path,
									Type: &volumeType,
								},
							},
						},
					},
					NodeSelector: cr.Spec.Workload.NodeSelector,
					Tolerations:  cr.Spec.Workload.Tolerations,
					Affinity:     cr.Spec.Workload.Affinity,
				},
			},
			RevisionHistoryLimit: pointer.Int32Ptr(10),
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "10%",
					},
					MaxSurge: &intstr.IntOrString{},
				},
			},
		},
	}
}

func getUsePrefix(cr *hostpathprovisionerv1.HostPathProvisioner) bool {
	if cr.Spec.PathConfig != nil {
		return cr.Spec.PathConfig.UseNamingPrefix
	}
	return false
}

func getPath(cr *hostpathprovisionerv1.HostPathProvisioner) string {
	if cr.Spec.PathConfig != nil {
		return cr.Spec.PathConfig.Path
	} else if len(cr.Spec.StoragePools) > 0 {
		if cr.Spec.StoragePools[0].Path != "" {
			return cr.Spec.StoragePools[0].Path
		}
	}
	return ""
}

func getStoragePoolPaths(cr *hostpathprovisionerv1.HostPathProvisioner) []StoragePoolInfo {
	storagePoolPaths := make([]StoragePoolInfo, 0)
	if cr.Spec.PathConfig != nil {
		storagePoolPaths = append(storagePoolPaths, StoragePoolInfo{
			Name: "",
			Path: cr.Spec.PathConfig.Path,
		})
	} else if len(cr.Spec.StoragePools) > 0 {
		for _, storagePool := range cr.Spec.StoragePools {
			storagePoolPaths = append(storagePoolPaths, StoragePoolInfo{
				Name:             storagePool.Name,
				Path:             storagePool.Path,
				SnapshotPath:     storagePool.SnapshotPath,
				SnapshotProvider: storagePool.SnapshotProvider,
			})
		}
	}
	return storagePoolPaths
}

func getMountNameFromStoragePool(poolName string) string {
	if poolName == "" {
		poolName = "csi"
	}
	return getResourceNameWithMaxLength(poolName, "data-dir", maxMountNameLength)
}

func buildPathArgFromStoragePoolInfo(storagePools []StoragePoolInfo) string {
	for i, storagePool := range storagePools {
		if storagePool.Name == "" {
			storagePools[i].Name = legacyStoragePoolName
		}
		// We want to add /csi to the path so if we are running side by side with legacy provisioner
		// the two paths don't mix.
		storagePools[i].Path = filepath.Join(getMountNameFromStoragePool(storagePool.Name), "csi")
		if storagePool.SnapshotPath != "" {
			storagePools[i].SnapshotPath = filepath.Join(getMountNameFromStoragePool(storagePool.Name), "snapshots")
		}
		storagePools[i].SnapshotProvider = storagePool.SnapshotProvider
	}
	bytes, err := json.Marshal(storagePools)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func buildVolumesFromStoragePoolInfo(storagePools []StoragePoolInfo) []corev1.Volume {
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	volumes := make([]corev1.Volume, 0)
	for _, storagePool := range storagePools {
		volumes = append(volumes, corev1.Volume{
			Name: getMountNameFromStoragePool(storagePool.Name),
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: storagePool.Path,
					Type: &directoryOrCreate,
				},
			},
		})
	}
	return volumes
}

func buildVolumeMountsFromStoragePoolInfo(storagePools []StoragePoolInfo) []corev1.VolumeMount {
	hostToContainer := corev1.MountPropagationHostToContainer
	mounts := make([]corev1.VolumeMount, 0)
	for _, storagePool := range storagePools {
		mountName := getMountNameFromStoragePool(storagePool.Name)
		mounts = append(mounts, corev1.VolumeMount{
			Name:             mountName,
			MountPath:        fmt.Sprintf("/%s", mountName),
			MountPropagation: &hostToContainer,
		})
	}
	return mounts
}

func (r *ReconcileHostPathProvisioner) createCSIDaemonSetObject(cr *hostpathprovisionerv1.HostPathProvisioner, reqLogger logr.Logger, args *daemonSetArgs) *appsv1.DaemonSet {
	reqLogger.V(3).Info("CR nodeselector", "nodeselector", cr.Spec.Workload)
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	directory := corev1.HostPathDirectory
	storagePoolPaths := getStoragePoolPaths(cr)
	pathVolumes := buildVolumesFromStoragePoolInfo(storagePoolPaths)
	pathMounts := buildVolumeMountsFromStoragePoolInfo(storagePoolPaths)
	biDirectional := corev1.MountPropagationBidirectional
	labels := util.GetRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue
	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.name,
			Namespace: args.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "10%",
					},
					MaxSurge: &intstr.IntOrString{},
				},
			},
			RevisionHistoryLimit: pointer.Int32Ptr(10),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						secv1.RequiredSCCAnnotation: "hostpath-provisioner-csi",
					},
				},
				Spec: corev1.PodSpec{

					ServiceAccountName: ProvisionerServiceAccountNameCsi,
					RestartPolicy:      corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("150Mi"),
								},
							},
							Name:            MultiPurposeHostPathProvisionerName,
							Image:           args.provisionerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  "CSI_ENDPOINT",
									Value: fmt.Sprintf("unix://%s", csiSocket),
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
								{
									Name:  "PV_DIR",
									Value: buildPathArgFromStoragePoolInfo(storagePoolPaths),
								},
								{
									Name:  "VERSION",
									Value: cr.Status.TargetVersion,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							Args: []string{
								fmt.Sprintf("--drivername=%s", driverName),
								fmt.Sprintf("--v=%d", args.verbosity),
								"--endpoint=$(CSI_ENDPOINT)",
								"--nodeid=$(NODE_NAME)",
								"--version=$(VERSION)",
								"--datadir=$(PV_DIR)",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9898,
									Name:          "healthz",
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 8080,
									Name:          "metrics",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold:    5,
								InitialDelaySeconds: 10,
								TimeoutSeconds:      3,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.IntOrString{
											IntVal: 9898,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "plugins-dir",
									MountPath:        "/var/lib/kubelet/plugins",
									MountPropagation: &biDirectional,
								},
								{
									Name:             "mountpoint-dir",
									MountPath:        "/var/lib/kubelet/pods",
									MountPropagation: &biDirectional,
								},
								socketDirVolumeMount,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							Name: nodeDriverRegistrarName,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("150Mi"),
								},
							},
							Image:           args.nodeDriverRegistrarImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Args: []string{
								fmt.Sprintf("--v=%d", args.verbosity),
								fmt.Sprintf("--csi-address=%s", csiSocket),
								"--kubelet-registration-path=/var/lib/kubelet/plugins/csi-hostpath/csi.sock",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							Env: []corev1.EnvVar{
								{
									Name: "KUBE_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								socketDirVolumeMount,
								{
									Name:      "registration-dir",
									MountPath: "/registration",
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("150Mi"),
								},
							},
							Name:            "liveness-probe",
							Image:           args.livenessProbeImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Args: []string{
								fmt.Sprintf("--csi-address=%s", csiSocket),
								"--health-port=9898",
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							VolumeMounts: []corev1.VolumeMount{
								socketDirVolumeMount,
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("150Mi"),
								},
							},
							Name:            "csi-provisioner",
							Image:           args.csiProvisionerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Args: []string{
								fmt.Sprintf("--v=%d", args.verbosity),
								fmt.Sprintf("--csi-address=%s", csiSocket),
								"--feature-gates=Topology=true",
								"--enable-capacity=true",
								"--capacity-for-immediate-binding=true",
								"--extra-create-metadata=true",
								"--immediate-topology=false",
								"--strict-topology=true",
								"--node-deployment=true",
								"--default-fstype=xfs",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								socketDirVolumeMount,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					SecurityContext:               &corev1.PodSecurityContext{},
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					Volumes: []corev1.Volume{
						{
							Name: "socket-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins/csi-hostpath",
									Type: &directoryOrCreate,
								},
							},
						},
						{
							Name: "mountpoint-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/pods",
									Type: &directoryOrCreate,
								},
							},
						},
						{
							Name: "registration-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins_registry",
									Type: &directory,
								},
							},
						},
						{
							Name: "plugins-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins",
									Type: &directory,
								},
							},
						},
					},
					NodeSelector: cr.Spec.Workload.NodeSelector,
					Tolerations:  cr.Spec.Workload.Tolerations,
					Affinity:     cr.Spec.Workload.Affinity,
				},
			},
		},
	}
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, pathVolumes...)
	if r.isFeatureGateEnabled(snapshotFeatureGate, cr) {
		ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, *createSnapshotSideCarContainer(args.snapshotterImage, cr.Spec.ImagePullPolicy, args.verbosity))
	}
	for i, container := range ds.Spec.Template.Spec.Containers {
		if container.Name == MultiPurposeHostPathProvisionerName || container.Name == nodeDriverRegistrarName {
			ds.Spec.Template.Spec.Containers[i].VolumeMounts = append(ds.Spec.Template.Spec.Containers[i].VolumeMounts, pathMounts...)
		}
	}

	return ds
}

func createSnapshotSideCarContainer(image string, pullPolicy corev1.PullPolicy, verbosity int) *corev1.Container {
	return &corev1.Container{
		Name:            "csi-snapshotter",
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Args: []string{
			fmt.Sprintf("--v=%d", verbosity),
			fmt.Sprintf("--csi-address=%s", csiSocket),
			"--leader-election",
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			socketDirVolumeMount,
		},
	}
}

// getDuplicateDaemonSet will give us duplicate DaemonSets from a previous version if they exist.
// This is possible from a previous HPP version where the resources (DaemonSet, RBAC) were named depending on the CR, whereas now, we have fixed names for those.
func (r *ReconcileHostPathProvisioner) getDuplicateDaemonSet(customCrName, namespace string) ([]appsv1.DaemonSet, error) {
	dsList := &appsv1.DaemonSetList{}
	dups := make([]appsv1.DaemonSet, 0)

	ls, err := k8slabels.Parse(fmt.Sprintf("k8s-app in (%s, %s)", MultiPurposeHostPathProvisionerName, customCrName))
	if err != nil {
		return dups, err
	}
	lo := &client.ListOptions{LabelSelector: ls, Namespace: namespace}
	if err := r.client.List(context.TODO(), dsList, lo); err != nil {
		return dups, err
	}

	for _, ds := range dsList.Items {
		if ds.Name != MultiPurposeHostPathProvisionerName && ds.Name != fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName) {
			for _, ownerRef := range ds.OwnerReferences {
				if ownerRef.Kind == "HostPathProvisioner" && ownerRef.Name == customCrName {
					dups = append(dups, ds)
					break
				}
			}
		}
	}

	return dups, nil
}
