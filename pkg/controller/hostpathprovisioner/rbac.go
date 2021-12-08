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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) reconcileClusterRoleBinding(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Define a new ClusterRoleBinding object
	if err := r.reconcileRbacResource(reqLogger.WithName("Provisioner RBAC"), createClusterRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), createClusterRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), cr); err != nil {
		return reconcile.Result{}, err
	}
	if r.isLegacy(cr) {
		if err := r.reconcileRbacResource(reqLogger.WithName("Provisioner RBAC"), createClusterRoleBindingObject(MultiPurposeHostPathProvisionerName, namespace, ProvisionerServiceAccountName), createClusterRoleBindingObject(MultiPurposeHostPathProvisionerName, namespace, ProvisionerServiceAccountName), cr); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		if err := r.deleteClusterRoleBindingObject(MultiPurposeHostPathProvisionerName); err != nil && !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) reconcileRbacResource(reqLogger logr.Logger, desired, found client.Object, cr *hostpathprovisionerv1.HostPathProvisioner) error {
	setLastAppliedConfiguration(desired)
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(found), found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Rbac Resource", "Name", desired.GetName())
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.GetName(), err))
			return err
		}

		// Resource created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.GetName()))
		return nil
	} else if err != nil {
		return err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged ClusterRoleBinding from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return err
	}

	// Rbac resource already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating Rbac resouce", "Name", desired.GetName())
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.GetName(), err))
			return err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.GetName()))
		return nil
	}

	// Rbac resource already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: Rbac Resource already exists", "Name", found.GetName())
	return nil
}

func createClusterRoleBindingObject(name, namespace, saName string) *rbacv1.ClusterRoleBinding {
	labels := getRecommendedLabels()
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (r *ReconcileHostPathProvisioner) deleteClusterRoleBindingObject(name string) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.client.Delete(context.TODO(), crb); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileHostPathProvisioner) reconcileClusterRole(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner) (reconcile.Result, error) {
	if r.isLegacy(cr) {
		if err := r.reconcileRbacResource(reqLogger.WithName("Provisioner RBAC"), createClusterRoleObjectProvisioner(), createClusterRoleObjectProvisioner(), cr); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		if err := r.deleteClusterRoleObject(MultiPurposeHostPathProvisionerName); err != nil && !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	}
	if err := r.reconcileRbacResource(reqLogger.WithName("Provisioner RBAC"), r.createCsiClusterRoleObjectProvisioner(cr), r.createCsiClusterRoleObjectProvisioner(cr), cr); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func createClusterRoleObjectProvisioner() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   MultiPurposeHostPathProvisionerName,
			Labels: getRecommendedLabels(),
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
					"watch",
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
					"patch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"nodes",
				},
				Verbs: []string{
					"get",
				},
			},
		},
	}
}

func (r *ReconcileHostPathProvisioner) createCsiClusterRoleObjectProvisioner(cr *hostpathprovisionerv1.HostPathProvisioner) *rbacv1.ClusterRole {
	res := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ProvisionerServiceAccountNameCsi,
			Labels: getRecommendedLabels(),
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
					"watch",
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
					"patch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"csinodes",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"nodes",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"volumeattachments",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"patch",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"volumeattachments/status",
				},
				Verbs: []string{
					"patch",
				},
			},
		},
	}
	if r.isFeatureGateEnabled(snapshotFeatureGate, cr) {
		res.Rules = append(res.Rules, createSnapshotCsiClusterRoles()...)
	}
	return res
}

func createSnapshotCsiClusterRoles() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"snapshot.storage.k8s.io",
			},
			Resources: []string{
				"volumesnapshotclasses",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"snapshot.storage.k8s.io",
			},
			Resources: []string{
				"volumesnapshots",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"snapshot.storage.k8s.io",
			},
			Resources: []string{
				"volumesnapshotcontents",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"snapshot.storage.k8s.io",
			},
			Resources: []string{
				"volumesnapshotcontents/status",
			},
			Verbs: []string{
				"update",
				"patch",
			},
		},
	}
}

func (r *ReconcileHostPathProvisioner) deleteClusterRoleObject(name string) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.client.Delete(context.TODO(), role); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileHostPathProvisioner) reconcileRoleBinding(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	if err := r.reconcileRbacResource(reqLogger.WithName("Provisioner RBAC"), createRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), createRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), cr); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func createRoleBindingObject(name, namespace, saName string) *rbacv1.RoleBinding {
	labels := getRecommendedLabels()
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (r *ReconcileHostPathProvisioner) reconcileRole(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	if err := r.reconcileRbacResource(reqLogger.WithName("provisioner RBAC"), createRoleObjectProvisioner(namespace), createRoleObjectProvisioner(namespace), cr); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func createRoleObjectProvisioner(namespace string) *rbacv1.Role {
	labels := getRecommendedLabels()
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProvisionerServiceAccountNameCsi,
			Namespace: namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"coordination.k8s.io",
				},
				Resources: []string{
					"leases",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"delete",
					"update",
					"create",
				},
			},
			{
				APIGroups: []string{
					"storage.k8s.io",
				},
				Resources: []string{
					"csistoragecapacities",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"delete",
					"update",
					"create",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
				},
				Verbs: []string{
					"get",
				},
			},
		},
	}
}

func (r *ReconcileHostPathProvisioner) deleteRoleBindingObject(name, namespace string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := r.client.Delete(context.TODO(), rb); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileHostPathProvisioner) deleteRoleObject(name, namespace string) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := r.client.Delete(context.TODO(), role); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
