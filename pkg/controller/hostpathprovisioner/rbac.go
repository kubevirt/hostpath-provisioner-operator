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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) reconcileClusterRoleBinding(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	var result reconcile.Result
	var err error
	// Define a new ClusterRoleBinding object
	result, err = r.reconcileClusterRoleBindingForSa(reqLogger.WithName("Provisioner RBAC"), createClusterRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), cr, namespace, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileClusterRoleBindingForSa(reqLogger.WithName("Health Check RBAC"), createClusterRoleBindingObject(healthCheckName, namespace, ProvisionerServiceAccountNameCsi), cr, namespace, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileClusterRoleBindingForSa(reqLogger.WithName("Provisioner RBAC"), createClusterRoleBindingObject(MultiPurposeHostPathProvisionerName, namespace, ProvisionerServiceAccountName), cr, namespace, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	return result, nil
}

func (r *ReconcileHostPathProvisioner) reconcileClusterRoleBindingForSa(reqLogger logr.Logger, desired *rbacv1.ClusterRoleBinding, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	setLastAppliedConfiguration(desired)
	// Check if this ClusterRoleBinding already exists
	found := &rbacv1.ClusterRoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// ClusterRoleBinding created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

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
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// ClusterRoleBinding already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: ClusterRoleBinding already exists", "ClusterRoleBinding.Name", found.Name)
	return reconcile.Result{}, nil
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

func (r *ReconcileHostPathProvisioner) reconcileClusterRole(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, recorder record.EventRecorder) (reconcile.Result, error) {
	result, err := r.reconcileClusterRoleForSa(reqLogger.WithName("Provisioner RBAC"), createClusterRoleObjectProvisioner(), cr, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileClusterRoleForSa(reqLogger.WithName("Provisioner RBAC"), r.createCsiClusterRoleObjectProvisioner(cr), cr, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileClusterRoleForSa(reqLogger.WithName("Provisioner RBAC"), createClusterRoleObjectHealthCheck(), cr, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	return result, nil
}

func (r *ReconcileHostPathProvisioner) reconcileClusterRoleForSa(reqLogger logr.Logger, desired *rbacv1.ClusterRole, cr *hostpathprovisionerv1.HostPathProvisioner, recorder record.EventRecorder) (reconcile.Result, error) {
	setLastAppliedConfiguration(desired)

	// Check if this ClusterRole already exists
	found := &rbacv1.ClusterRole{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ClusterRole", "ClusterRole.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// ClusterRole created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

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
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// ClusterRole already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: ClusterRole already exists", "ClusterRole.Name", found.Name)
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

func createClusterRoleObjectHealthCheck() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   healthCheckName,
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
					"",
				},
				Resources: []string{
					"pods",
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
					"get",
					"list",
					"watch",
					"create",
					"patch",
				},
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

func (r *ReconcileHostPathProvisioner) reconcileRoleBinding(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	result, err := r.reconcileRoleBindingForSa(reqLogger.WithName("Provisioner RBAC"), createRoleBindingObject(ProvisionerServiceAccountNameCsi, namespace, ProvisionerServiceAccountNameCsi), cr, namespace, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileRoleBindingForSa(reqLogger.WithName("Health Check RBAC"), createRoleBindingObject(healthCheckName, namespace, ProvisionerServiceAccountNameCsi), cr, namespace, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	return result, nil
}

func (r *ReconcileHostPathProvisioner) reconcileRoleBindingForSa(reqLogger logr.Logger, desired *rbacv1.RoleBinding, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	setLastAppliedConfiguration(desired)
	// Check if this RoleBinding already exists
	found := &rbacv1.RoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new RoleBinding", "RoleBinding.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// RoleBinding created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged ClusterRoleBinding from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// RoleBinding already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating RoleBinding", "RoleBinding.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// RoleBinding already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: RoleBinding already exists", "RoleBinding.Name", found.Name)
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

func (r *ReconcileHostPathProvisioner) reconcileRole(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	result, err := r.reconcileRoleForSa(reqLogger.WithName("provisioner RBAC"), createRoleObjectProvisioner(namespace), cr, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	result, err = r.reconcileRoleForSa(reqLogger.WithName("healthcheck RBAC"), createRoleObjectHealthCheck(namespace), cr, recorder)
	if err != nil {
		return reconcile.Result{}, err
	}
	return result, nil
}

func (r *ReconcileHostPathProvisioner) reconcileRoleForSa(reqLogger logr.Logger, desired *rbacv1.Role, cr *hostpathprovisionerv1.HostPathProvisioner, recorder record.EventRecorder) (reconcile.Result, error) {
	setLastAppliedConfiguration(desired)

	// Check if this Role already exists
	found := &rbacv1.Role{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Role", "Role.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// Role created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged Role from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Role already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating Role", "Role.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// Role already exists and matches the desired state - don't requeue
	reqLogger.V(3).Info("Skip reconcile: Role already exists", "Role.Name", found.Name)
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

func createRoleObjectHealthCheck(namespace string) *rbacv1.Role {
	labels := getRecommendedLabels()
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      healthCheckName,
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
