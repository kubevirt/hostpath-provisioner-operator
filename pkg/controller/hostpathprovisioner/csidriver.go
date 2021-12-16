/*
Copyright 2021 The hostpath provisioner operator Authors.

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
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

const (
	driverName = "kubevirt.io.hostpath-provisioner"
)

func (r *ReconcileHostPathProvisioner) reconcileCSIDriver(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Define a new CSIDriver object
	desired := createCSIDriverObject(namespace)

	setLastAppliedConfiguration(desired)

	// Check if this CSIDriver already exists
	found := &storagev1.CSIDriver{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: driverName}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new CSI Driver", "CSIDriver.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// CSIDriver created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// Copy some immutable spec fields on csidrivers that don't need to be reconciled
	desired = copyImmutableFields(desired, found)

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged CSIDriver from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// CSIDriver already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating CSIDriver", "CSIDriver.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	// CSIDriver already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: CSIDriver already exists", "CSIDriver.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deleteCSIDriver() error {
	// Check if this CSIDriver already exists
	csiDriver := &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: driverName,
		},
	}

	if err := r.client.Delete(context.TODO(), csiDriver); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func copyImmutableFields(desired, current *storagev1.CSIDriver) *storagev1.CSIDriver {
	desired.Spec.AttachRequired = current.Spec.AttachRequired
	desired.Spec.PodInfoOnMount = current.Spec.PodInfoOnMount
	desired.Spec.VolumeLifecycleModes = current.Spec.VolumeLifecycleModes
	desired.Spec.StorageCapacity = current.Spec.StorageCapacity
	desired.Spec.FSGroupPolicy = current.Spec.FSGroupPolicy

	return desired
}

func createCSIDriverObject(namespace string) *storagev1.CSIDriver {
	labels := getRecommendedLabels()
	podInfoOnMount := true
	attachRequired := false
	storageCapacity := true
	requiresRepublish := false
	fsGroupPolicy := storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy

	return &storagev1.CSIDriver{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "CSIDriver",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   driverName,
			Labels: labels,
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: &attachRequired,
			FSGroupPolicy:  &fsGroupPolicy,
			VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{
				storagev1.VolumeLifecyclePersistent,
				storagev1.VolumeLifecycleEphemeral,
			},
			PodInfoOnMount:    &podInfoOnMount,
			StorageCapacity:   &storageCapacity,
			RequiresRepublish: &requiresRepublish,
		},
	}
}
