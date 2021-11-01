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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	csiSocket = "/csi/csi.sock"
)

var (
	socketDirVolumeMount = corev1.VolumeMount{Name: "socket-dir", MountPath: "/csi"}
	dataDirVolumeMount   = corev1.VolumeMount{Name: "csi-data-dir", MountPath: "/csi-data-dir"}
)

type daemonSetArgs struct {
	provisionerImage                     string
	externalHealthMonitorControllerImage string
	nodeDriverRegistrarImage             string
	livenessProbeImage                   string
	snapshotterImage                     string
	csiProvisionerImage                  string
	namespace                            string
	name                                 string
	verbosity                            int
	version                              string
}

// reconcileDaemonSet Reconciles the daemon set.
func (r *ReconcileHostPathProvisioner) reconcileDaemonSet(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
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
	// provisioner
	args := getDaemonSetArgs(reqLogger.WithName("daemonset args"), namespace, true)
	args.version = cr.Status.TargetVersion
	if res, err := r.reconcileDaemonSetForSa(reqLogger, createDaemonSetObject(cr, reqLogger, args), cr, namespace, recorder); err != nil {
		return res, err
	}
	// csi driver
	args = getDaemonSetArgs(reqLogger.WithName("daemonset args"), namespace, false)
	args.version = cr.Status.TargetVersion
	return r.reconcileDaemonSetForSa(reqLogger, r.createCSIDaemonSetObject(cr, reqLogger, args), cr, namespace, recorder)
}

func (r *ReconcileHostPathProvisioner) reconcileDaemonSetForSa(reqLogger logr.Logger, desired *appsv1.DaemonSet, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
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
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// DaemonSet created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
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
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
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

		res.externalHealthMonitorControllerImage = os.Getenv(externalHealthMonitorControllerImageEnvVarName)
		if res.externalHealthMonitorControllerImage == "" {
			reqLogger.V(3).Info(fmt.Sprintf("%s not set, defaulting to %s", externalHealthMonitorControllerImageEnvVarName, CsiExternalHealthMonitorControllerImageDefault))
			res.externalHealthMonitorControllerImage = CsiExternalHealthMonitorControllerImageDefault
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
	labels := getRecommendedLabels()
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
				MatchLabels: labels,
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
							Name:            MultiPurposeHostPathProvisionerName,
							Image:           args.provisionerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  "USE_NAMING_PREFIX",
									Value: strconv.FormatBool(cr.Spec.PathConfig.UseNamingPrefix),
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
									Value: cr.Spec.PathConfig.Path,
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
									MountPath: cr.Spec.PathConfig.Path,
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
									Path: cr.Spec.PathConfig.Path,
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

func (r *ReconcileHostPathProvisioner) createCSIDaemonSetObject(cr *hostpathprovisionerv1.HostPathProvisioner, reqLogger logr.Logger, args *daemonSetArgs) *appsv1.DaemonSet {
	reqLogger.V(3).Info("CR nodeselector", "nodeselector", cr.Spec.Workload)
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	directory := corev1.HostPathDirectory
	biDirectional := corev1.MountPropagationBidirectional
	labels := getRecommendedLabels()
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
				MatchLabels: labels,
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
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ProvisionerServiceAccountNameCsi,
					RestartPolicy:      corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
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
									Value: filepath.Join(cr.Spec.PathConfig.Path, "csi"),
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
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9898,
									Name:          "healthz",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold:    5,
								InitialDelaySeconds: 10,
								TimeoutSeconds:      3,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								Handler: corev1.Handler{
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
								dataDirVolumeMount,
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
							Name:            "csi-external-health-monitor-controller",
							Image:           args.externalHealthMonitorControllerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Args: []string{
								fmt.Sprintf("--v=%d", args.verbosity),
								"--csi-address=$(ADDRESS)",
								"--leader-election",
							},
							VolumeMounts: []corev1.VolumeMount{
								socketDirVolumeMount,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ADDRESS",
									Value: csiSocket,
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
							Name:            "node-driver-registrar",
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
								dataDirVolumeMount,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
						{
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
							Name: "csi-data-dir", // Has to match VolumeMounts in containers
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: cr.Spec.PathConfig.Path,
									Type: &directoryOrCreate,
								},
							},
						},
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
	if r.isFeatureGateEnabled(snapshotFeatureGate, cr) {
		ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, *createSnapshotSideCarContainer(args.snapshotterImage, cr.Spec.ImagePullPolicy, args.verbosity))
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

	ls, err := labels.Parse(fmt.Sprintf("k8s-app in (%s, %s)", MultiPurposeHostPathProvisionerName, customCrName))
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
