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
	"os"
	"reflect"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	hostpathprovisionerv1alpha1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcileDaemonSet Reconciles the daemon set.
func (r *ReconcileHostPathProvisioner) reconcileDaemonSet(reqLogger logr.Logger, instance *hostpathprovisionerv1alpha1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Define a new DaemonSet object
	provisionerImage := os.Getenv(provisionerImageEnvVarName)
	if provisionerImage == "" {
		reqLogger.Info("PROVISIONER_IMAGE not set, defaulting to hostpath-provisioner")
		provisionerImage = ProvisionerImageDefault
	}

	desired := createDaemonSetObject(instance, provisionerImage, namespace)
	desiredMetaObj := &desired.ObjectMeta
	setLastAppliedConfiguration(desiredMetaObj)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this DaemonSet already exists
	found := &appsv1.DaemonSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DaemonSet", "DaemonSet.Namespace", desired.Namespace, "Daemonset.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			return reconcile.Result{}, err
		}

		// DaemonSet created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()
	// Copy found status fields, so the compare won't fail on desired/scheduled/ready pods being different. Updating will ignore them anyway.
	desired = copyStatusFields(desired, found)

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desiredMetaObj, &found.ObjectMeta)

	// create merged DaemonSet from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating DaemonSet", "DaemonSet.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// DaemonSet already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: DaemonSet already exists", "DaemonSet.Namespace", found.Namespace, "Daemonset.Name", found.Name)
	return reconcile.Result{}, nil
}

func copyStatusFields(desired, current *appsv1.DaemonSet) *appsv1.DaemonSet {
	desired.Status = *current.Status.DeepCopy()
	return desired
}

// createDaemonSetObject returns a new DaemonSet in the same namespace as the cr
func createDaemonSetObject(cr *hostpathprovisionerv1alpha1.HostPathProvisioner, provisionerImage, namespace string) *appsv1.DaemonSet {
	volumeType := corev1.HostPathDirectoryOrCreate
	labels := map[string]string{
		"k8s-app": cr.Name,
	}
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": cr.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cr.Name + "-admin",
					Containers: []corev1.Container{
						{
							Name:            cr.Name,
							Image:           provisionerImage,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  "USE_NAMING_PREFIX",
									Value: cr.Spec.PathConfig.UseNamingPrefix,
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
				},
			},
		},
	}
}
