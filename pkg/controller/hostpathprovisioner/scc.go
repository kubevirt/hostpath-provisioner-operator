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
	secv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) reconcileSecurityContextConstraints(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	if used, err := r.checkSCCUsed(); used == false {
		return reconcile.Result{}, err
	}

	// Define a new SecurityContextConstraints object
	desired := createSecurityContextConstraintsObject(cr, namespace)
	desiredMetaObj := &desired.ObjectMeta
	setLastAppliedConfiguration(desiredMetaObj)

	// Check if this SecurityContextConstraints already exists
	found := &secv1.SecurityContextConstraints{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SecurityContextConstraints", "SecurityContextConstraints.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			return reconcile.Result{}, err
		}
		// SecurityContextConstraints created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desiredMetaObj, &found.ObjectMeta)

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
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	// SecurityContextConstraints already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: SecurityContextConstraints already exists", "SecurityContextConstraints.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deleteSCC(cr *hostpathprovisionerv1.HostPathProvisioner) error {
	if used, err := r.checkSCCUsed(); used == false {
		return err
	}
	// Check if this SecurityContextConstraints already exists
	scc := &secv1.SecurityContextConstraints{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name}, scc)
	if err != nil && errors.IsNotFound(err) {
		// Already gone, return
		return nil
	} else if err != nil {
		return err
	}
	// Delete SCC
	return r.client.Delete(context.TODO(), scc)
}

func createSecurityContextConstraintsObject(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *secv1.SecurityContextConstraints {
	saName := fmt.Sprintf("system:serviceaccount:%s:%s-admin", namespace, cr.Name)
	labels := map[string]string{
		"k8s-app": cr.Name,
	}
	return &secv1.SecurityContextConstraints{
		Groups: []string{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   cr.Name,
			Labels: labels,
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
		Volumes: []secv1.FSType{
			secv1.FSTypeHostPath,
			secv1.FSTypeSecret,
		},
	}
}

func (r *ReconcileHostPathProvisioner) checkSCCUsed() (bool, error) {
	// Check if we are using security context constraints, if not return false.
	listObj := &secv1.SecurityContextConstraintsList{}
	if err := r.client.List(context.TODO(), listObj); err != nil {
		if meta.IsNoMatchError(err) {
			// not using SCCs
			return false, nil
		}
		return false, err
	}
	return true, nil
}
