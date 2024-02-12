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
	"strings"

	"github.com/go-logr/logr"
	secv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/pkg/util"
)

func (r *ReconcileHostPathProvisioner) reconcileSecurityContextConstraints(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	if used, err := r.checkSCCUsed(); err != nil {
		return reconcile.Result{}, err
	} else if used == false {
		return reconcile.Result{}, nil
	}
	if r.isLegacy(cr) {
		if res, err := r.reconcileSecurityContextConstraintsDesired(reqLogger, cr, createSecurityContextConstraintsObject(namespace)); err != nil {
			return res, err
		}
	} else {
		if err := r.deleteSCC(MultiPurposeHostPathProvisionerName); err != nil {
			return reconcile.Result{}, err
		}
	}
	return r.reconcileSecurityContextConstraintsDesired(reqLogger, cr, createCsiSecurityContextConstraintsObject(namespace))
}

func (r *ReconcileHostPathProvisioner) reconcileSecurityContextConstraintsDesired(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired *secv1.SecurityContextConstraints) (reconcile.Result, error) {
	// Define a new SecurityContextConstraints object
	setLastAppliedConfiguration(desired)

	// Check if this SecurityContextConstraints already exists
	found := &secv1.SecurityContextConstraints{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SecurityContextConstraints", "SecurityContextConstraints.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// SecurityContextConstraints created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged SecurityContextConstraints from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// SecurityContextConstraints already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating SecurityContextConstraints", "SecurityContextConstraints.Name", desired.Name)
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}
	// SecurityContextConstraints already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: SecurityContextConstraints already exists", "SecurityContextConstraints.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deleteSCC(name string) error {
	if used, err := r.checkSCCUsed(); used == false {
		return err
	}
	// Check if this SecurityContextConstraints already exists
	scc := &secv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.client.Delete(context.TODO(), scc); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func createSecurityContextConstraintsObject(namespace string) *secv1.SecurityContextConstraints {
	saName := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, ProvisionerServiceAccountName)
	res := &secv1.SecurityContextConstraints{
		Groups: []string{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   MultiPurposeHostPathProvisionerName,
			Labels: util.GetRecommendedLabels(),
		},
		AllowPrivilegedContainer: false,
		RequiredDropCapabilities: []corev1.Capability{
			"KILL",
			"MKNOD",
			"SETUID",
			"SETGID",
		},
		RunAsUser: secv1.RunAsUserStrategyOptions{
			Type: secv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: secv1.SELinuxContextStrategyOptions{
			Type: secv1.SELinuxStrategyRunAsAny,
		},
		FSGroup: secv1.FSGroupStrategyOptions{
			Type: secv1.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: secv1.SupplementalGroupsStrategyOptions{
			Type: secv1.SupplementalGroupsStrategyRunAsAny,
		},
		AllowHostDirVolumePlugin: true,
		Users: []string{
			saName,
		},
	}
	res.Volumes = []secv1.FSType{
		secv1.FSTypeHostPath,
		secv1.FSTypeSecret,
		secv1.FSProjected,
	}
	return res
}

func createCsiSecurityContextConstraintsObject(namespace string) *secv1.SecurityContextConstraints {
	saName := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, ProvisionerServiceAccountNameCsi)
	return &secv1.SecurityContextConstraints{
		Groups: []string{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Labels: util.GetRecommendedLabels(),
		},
		AllowPrivilegedContainer: true,
		RequiredDropCapabilities: []corev1.Capability{
			"KILL",
			"MKNOD",
			"SETUID",
			"SETGID",
		},
		RunAsUser: secv1.RunAsUserStrategyOptions{
			Type: secv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: secv1.SELinuxContextStrategyOptions{
			Type: secv1.SELinuxStrategyRunAsAny,
		},
		FSGroup: secv1.FSGroupStrategyOptions{
			Type: secv1.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: secv1.SupplementalGroupsStrategyOptions{
			Type: secv1.SupplementalGroupsStrategyRunAsAny,
		},
		AllowHostDirVolumePlugin: true,
		Users: []string{
			saName,
		},
		Volumes: []secv1.FSType{
			secv1.FSTypeAll,
		},
	}
}

func (r *ReconcileHostPathProvisioner) checkSCCUsed() (bool, error) {
	// Check if we are using security context constraints, if not return false.
	listObj := &secv1.SecurityContextConstraintsList{}
	if err := r.client.List(context.TODO(), listObj); err != nil {
		if meta.IsNoMatchError(err) || strings.Contains(err.Error(), "failed to find API group") {
			// not using SCCs
			return false, nil
		}
		return false, err
	}
	return true, nil
}
