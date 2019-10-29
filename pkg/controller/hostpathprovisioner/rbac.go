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
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	hostpathprovisionerv1alpha1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1alpha1"
)

func (r *ReconcileHostPathProvisioner) reconcileClusterRoleBinding(reqLogger logr.Logger, instance *hostpathprovisionerv1alpha1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Define a new ClusterRoleBinding object
	desired := createClusterRoleBindingObject(instance, namespace)
	desiredMetaObj := &desired.ObjectMeta
	setLastAppliedConfiguration(desiredMetaObj)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this ClusterRoleBinding already exists
	found := &rbacv1.ClusterRoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			return reconcile.Result{}, err
		}

		// ClusterRoleBinding created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desiredMetaObj, &found.ObjectMeta)

	// create merged ClusterRoleBinding from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// ClusterRoleBinding already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// ClusterRoleBinding already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: ClusterRoleBinding already exists", "ClusterRoleBinding.Name", found.Name)
	return reconcile.Result{}, nil
}

func createClusterRoleBindingObject(cr *hostpathprovisionerv1alpha1.HostPathProvisioner, namespace string) *rbacv1.ClusterRoleBinding {
	labels := map[string]string{
		"k8s-app": cr.Name,
	}
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cr.Name,
			Labels: labels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      cr.Name + "-admin",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     cr.Name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (r *ReconcileHostPathProvisioner) reconcileClusterRole(reqLogger logr.Logger, instance *hostpathprovisionerv1alpha1.HostPathProvisioner) (reconcile.Result, error) {
	// Define a new ClusterRole object
	desired := createClusterRoleObject(instance.Name)
	desiredMetaObj := &desired.ObjectMeta
	setLastAppliedConfiguration(desiredMetaObj)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this ClusterRole already exists
	found := &rbacv1.ClusterRole{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ClusterRole", "ClusterRole.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			return reconcile.Result{}, err
		}

		// ClusterRole created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desiredMetaObj, &found.ObjectMeta)

	// create merged ClusterRole from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// ClusterRole already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating ClusterRole", "ClusterRole.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// ClusterRole already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: ClusterRole already exists", "ClusterRole.Name", found.Name)
	return reconcile.Result{}, nil
}

func createClusterRoleObject(name string) *rbacv1.ClusterRole {
	labels := map[string]string{
		"k8s-app": name,
	}
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"persistentvolumes",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"delete",
					"get",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"persistentvolumeclaims",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"storageclasses",
				},
				Verbs: []string{
					"get",
					"list",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"storageclasses",
				},
				ResourceNames: []string{
					name,
				},
				Verbs: []string{
					"watch",
					"create",
					"update",
					"delete",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"events",
				},
				Verbs: []string{
					"list",
					"watch",
					"create",
				},
			},
		},
	}
}
