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
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

	// Define a new DaemonSet object
	provisionerImage := os.Getenv(provisionerImageEnvVarName)
	if provisionerImage == "" {
		reqLogger.Info("PROVISIONER_IMAGE not set, defaulting to hostpath-provisioner")
		provisionerImage = ProvisionerImageDefault
	}

	desired := createDaemonSetObject(cr, reqLogger, provisionerImage, namespace)
	setLastAppliedConfiguration(desired)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this DaemonSet already exists
	found := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DaemonSet", "DaemonSet.Namespace", desired.Namespace, "Daemonset.Name", desired.Name)
		recorder.Event(cr, corev1.EventTypeNormal, createResourceStart, fmt.Sprintf(createMessageStart, desired, desired.Name))
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
	desired = copyStatusFields(desired, found)

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	found.Spec = *desired.Spec.DeepCopy()

	if !reflect.DeepEqual(currentRuntimeObjCopy, found) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, found)
		// Current is different from desired, update.
		reqLogger.Info("Updating DaemonSet", "DaemonSet.Name", desired.Name)
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceStart, fmt.Sprintf(updateMessageStart, desired, desired.Name))
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

func copyStatusFields(desired, current *appsv1.DaemonSet) *appsv1.DaemonSet {
	desired.Status = *current.Status.DeepCopy()
	return desired
}

// createDaemonSetObject returns a new DaemonSet in the same namespace as the cr
func createDaemonSetObject(cr *hostpathprovisionerv1.HostPathProvisioner, reqLogger logr.Logger, provisionerImage, namespace string) *appsv1.DaemonSet {
	reqLogger.Info("CR nodeselector", "nodeselector", cr.Spec.Workloads)
	volumeType := corev1.HostPathDirectoryOrCreate
	labels := map[string]string{
		"k8s-app": MultiPurposeHostPathProvisionerName,
	}
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: namespace,
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
					ServiceAccountName: ControllerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            MultiPurposeHostPathProvisionerName,
							Image:           provisionerImage,
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
					NodeSelector: cr.Spec.Workloads.NodeSelector,
					Tolerations:  cr.Spec.Workloads.Tolerations,
					Affinity:     cr.Spec.Workloads.Affinity,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "10%",
					},
				},
			},
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
		if ds.Name != MultiPurposeHostPathProvisionerName {
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
