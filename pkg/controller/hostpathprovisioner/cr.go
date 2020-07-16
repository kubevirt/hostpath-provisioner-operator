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
	corev1 "k8s.io/api/core/v1"

	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) isDeploying(cr *hostpathprovisionerv1.HostPathProvisioner) bool {
	return cr.Status.ObservedVersion == ""
}

func (r *ReconcileHostPathProvisioner) isUpgrading(cr *hostpathprovisionerv1.HostPathProvisioner) bool {
	return cr.Status.ObservedVersion != "" && cr.Status.ObservedVersion != cr.Status.TargetVersion
}

// MarkCrHealthyMessage marks the passed in CR as healthy. The CR object needs to be updated by the caller afterwards.
// Healthy means the following status conditions are set:
// ApplicationAvailable: true
// Progressing: false
// Degraded: false
func MarkCrHealthyMessage(cr *hostpathprovisionerv1.HostPathProvisioner, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionAvailable,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionDegraded,
		Status: corev1.ConditionFalse,
	})
}

// IsCrHealthy returns true if the application is healthy.
// Healthy means the following status conditions are set:
// ApplicationAvailable: true
// Progressing: false
// Degraded: false
func IsCrHealthy(cr *hostpathprovisionerv1.HostPathProvisioner) bool {
	return conditions.IsStatusConditionTrue(cr.Status.Conditions, conditions.ConditionAvailable) &&
		conditions.IsStatusConditionFalse(cr.Status.Conditions, conditions.ConditionProgressing) &&
		conditions.IsStatusConditionFalse(cr.Status.Conditions, conditions.ConditionDegraded)
}

// MarkCrUpgradeHealingDegraded marks the passed CR as upgrading and degraded. The CR object needs to be updated by the caller afterwards.
// Failed means the following status conditions are set:
// ApplicationAvailable: true
// Progressing: true
// Degraded: true
func MarkCrUpgradeHealingDegraded(cr *hostpathprovisionerv1.HostPathProvisioner, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionAvailable,
		Status: corev1.ConditionTrue,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionTrue,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionDegraded,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
}

// MarkCrFailed marks the passed CR as failed and requiring human intervention. The CR object needs to be updated by the caller afterwards.
// Failed means the following status conditions are set:
// ApplicationAvailable: false
// Progressing: false
// Degraded: true
func MarkCrFailed(cr *hostpathprovisionerv1.HostPathProvisioner, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionAvailable,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionDegraded,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
}

// MarkCrFailedHealing marks the passed CR as failed and healing. The CR object needs to be updated by the caller afterwards.
// FailedAndHealing means the following status conditions are set:
// ApplicationAvailable: false
// Progressing: true
// Degraded: true
func MarkCrFailedHealing(cr *hostpathprovisionerv1.HostPathProvisioner, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionAvailable,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionTrue,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionDegraded,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
}

// MarkCrDeploying marks the passed CR as currently deploying. The CR object needs to be updated by the caller afterwards.
// Deploying means the following status conditions are set:
// ApplicationAvailable: false
// Progressing: true
// Degraded: false
func MarkCrDeploying(cr *hostpathprovisionerv1.HostPathProvisioner, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionAvailable,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionProgressing,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionDegraded,
		Status: corev1.ConditionFalse,
	})
}
