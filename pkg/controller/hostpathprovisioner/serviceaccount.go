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
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) reconcileServiceAccount(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Previous versions created resources with names that depend on the CR, whereas now, we have fixed names for those.
	// We will remove those and have the next loop create the resources with fixed names so we don't end up with two sets of hpp resources.
	dups, err := r.getDuplicateServiceAccount(cr.Name, namespace, cr.Spec.DisableCsi)
	if err != nil {
		return reconcile.Result{}, err
	}
	for _, dup := range dups {
		reqLogger.Info("Deleting extra service account", "namespace", namespace, "name", dup.Name)
		if err := r.deleteServiceAccount(dup.Name, namespace); err != nil {
			return reconcile.Result{}, err
		}
	}

	accounts := make([]*corev1.ServiceAccount, 0)
	accounts = append(accounts, createServiceAccountObject(namespace))
	accounts = append(accounts, createCsiServiceAccountObject(namespace))
	for _, desired := range accounts {
		// Define a new Service Account object
		setLastAppliedConfiguration(desired)

		// Set HostPathProvisioner instance as the owner and controller
		if err := controllerutil.SetControllerReference(cr, desired, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this ServiceAccount already exists
		found := &corev1.ServiceAccount{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Service Account", "ServiceAccount.Namespace", desired.Namespace, "ServiceAccount.Name", desired.Name)
			r.recorder.Event(cr, corev1.EventTypeNormal, createResourceStart, fmt.Sprintf(createMessageStart, desired, desired.Name))
			err = r.client.Create(context.TODO(), desired)
			if err != nil {
				r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
				return reconcile.Result{}, err
			}

			// Service Account created successfully - don't requeue
			r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
			continue
		} else if err != nil {
			return reconcile.Result{}, err
		}

		// Keep a copy of the original for comparison later.
		currentRuntimeObjCopy := found.DeepCopyObject()

		// allow users to add new annotations (but not change ours)
		mergeLabelsAndAnnotations(desired, found)

		// create merged ServiceAccount from found and desired.
		merged, err := mergeObject(desired, found)
		if err != nil {
			return reconcile.Result{}, err
		}

		// ServiceAccount already exists, check if we need to update.
		if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
			logJSONDiff(log, currentRuntimeObjCopy, merged)
			// Current is different from desired, update.
			reqLogger.Info("Updating Service Account", "ServiceAccount.Name", desired.Name)
			r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceStart, fmt.Sprintf(updateMessageStart, desired, desired.Name))
			err = r.client.Update(context.TODO(), merged)
			if err != nil {
				r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
				return reconcile.Result{}, err
			}
			r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
			continue
		}

		// Service Account already exists and matches desired - don't requeue
		reqLogger.V(3).Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deleteServiceAccount(name, namespace string) error {
	// Check if this ServiceAccount already exists
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if err := r.client.Delete(context.TODO(), sa); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// createServiceAccount returns a new Service Account object in the same namespace as the cr.
func createServiceAccountObject(namespace string) *corev1.ServiceAccount {
	labels := getRecommendedLabels()
	return createServiceAccount(ProvisionerServiceAccountName, namespace, labels)
}

// createServiceAccount returns a new Service Account object in the same namespace as the cr.
func createCsiServiceAccountObject(namespace string) *corev1.ServiceAccount {
	labels := getRecommendedLabels()
	return createServiceAccount(ProvisionerServiceAccountNameCsi, namespace, labels)
}

func createServiceAccount(name, namespace string, labels map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

// getDuplicateServiceAccount will give us duplicate ServiceAccounts from a previous version if they exist.
// This is possible from a previous HPP version where the resources (DaemonSet, RBAC) were named depending on the CR, whereas now, we have fixed names for those.
func (r *ReconcileHostPathProvisioner) getDuplicateServiceAccount(customCrName, namespace string, DisableCsi bool) ([]corev1.ServiceAccount, error) {
	saList := &corev1.ServiceAccountList{}
	dups := make([]corev1.ServiceAccount, 0)

	ls, err := labels.Parse(fmt.Sprintf("k8s-app in (%s, %s)", MultiPurposeHostPathProvisionerName, customCrName))
	if err != nil {
		return dups, err
	}
	lo := &client.ListOptions{LabelSelector: ls, Namespace: namespace}
	if err := r.client.List(context.TODO(), saList, lo); err != nil {
		return dups, err
	}

	for _, sa := range saList.Items {
		if sa.Name != ProvisionerServiceAccountName && sa.Name != healthCheckName && sa.Name != ProvisionerServiceAccountNameCsi {
			for _, ownerRef := range sa.OwnerReferences {
				if ownerRef.Kind == "HostPathProvisioner" && ownerRef.Name == customCrName {
					dups = append(dups, sa)
					break
				}
			}
		}
	}

	return dups, nil
}
