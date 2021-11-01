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

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

const (
	ruleName            = "prometheus-hpp-rules"
	rbacName            = "hpp-monitoring"
	monitorName         = "service-monitor-hpp"
	defaultMonitoringNs = "monitoring"
)

func (r *ReconcileHostPathProvisioner) reconcilePrometheusInfra(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	if used, err := r.checkPrometheusUsed(); err != nil {
		return reconcile.Result{}, err
	} else if used == false {
		return reconcile.Result{}, nil
	}
	if res, err := r.reconcilePrometheusRuleDesired(reqLogger, cr, createPrometheusRule(cr, namespace), namespace, recorder); err != nil {
		return res, err
	}
	if res, err := r.reconcilePrometheusRoleDesired(reqLogger, cr, createPrometheusRole(cr, namespace), namespace, recorder); err != nil {
		return res, err
	}
	if res, err := r.reconcilePrometheusRoleBindingDesired(reqLogger, cr, createPrometheusRoleBinding(cr, namespace), namespace, recorder); err != nil {
		return res, err
	}
	return r.reconcilePrometheusServiceMonitorDesired(reqLogger, cr, createPrometheusServiceMonitor(cr, namespace), namespace, recorder)
}

func (r *ReconcileHostPathProvisioner) reconcilePrometheusRuleDesired(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired *promv1.PrometheusRule, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	// Define a new PrometheusRule object
	setLastAppliedConfiguration(desired)

	// Check if this PrometheusRule already exists
	found := &promv1.PrometheusRule{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new PrometheusRule", "PrometheusRule.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// PrometheusRule created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// PrometheusRule already exists, check if we need to update.
	if !reflect.DeepEqual(found.Spec, desired.Spec) {
		logJSONDiff(reqLogger, found, desired)
		// Current is different from desired, update.
		reqLogger.Info("Updating PrometheusRule", "PrometheusRule.Name", desired.Name)
		found.Spec = desired.Spec
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}
	// PrometheusRule already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: PrometheusRule already exists", "PrometheusRule.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) reconcilePrometheusRoleDesired(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired *rbacv1.Role, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	// Define a new Prometheus Role object
	setLastAppliedConfiguration(desired)

	// Check if this Prometheus Role already exists
	found := &rbacv1.Role{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Prometheus Role", "Role.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// Prometheus Role created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Prometheus Role already exists, check if we need to update.
	if !reflect.DeepEqual(found.Rules, desired.Rules) {
		logJSONDiff(reqLogger, found, desired)
		// Current is different from desired, update.
		reqLogger.Info("Updating Prometheus Role", "Role.Name", desired.Name)
		found.Rules = desired.Rules
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}
	// Prometheus Role already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: Prometheus Role already exists", "Role.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) reconcilePrometheusRoleBindingDesired(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired *rbacv1.RoleBinding, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	// Define a new Prometheus RoleBinding object
	setLastAppliedConfiguration(desired)

	// Check if this Prometheus RoleBinding already exists
	found := &rbacv1.RoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Prometheus RoleBinding", "RoleBinding.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// Prometheus RoleBinding created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Prometheus RoleBinding already exists, check if we need to update.
	if !reflect.DeepEqual(found.Subjects, desired.Subjects) {
		logJSONDiff(reqLogger, found, desired)
		// Current is different from desired, update.
		reqLogger.Info("Updating Prometheus RoleBinding", "RoleBinding.Name", desired.Name)
		found.Subjects = desired.Subjects
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}
	// Prometheus RoleBinding already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: Prometheus RoleBinding already exists", "RoleBinding.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) reconcilePrometheusServiceMonitorDesired(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired *promv1.ServiceMonitor, namespace string, recorder record.EventRecorder) (reconcile.Result, error) {
	// Define a new ServiceMonitor object
	setLastAppliedConfiguration(desired)

	// Check if this ServiceMonitor already exists
	found := &promv1.ServiceMonitor{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Prometheus ServiceMonitor", "ServiceMonitor.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		// ServiceMonitor created successfully - don't requeue
		recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// ServiceMonitor already exists, check if we need to update.
	if !reflect.DeepEqual(found.Spec, desired.Spec) {
		logJSONDiff(reqLogger, found, desired)
		// Current is different from desired, update.
		reqLogger.Info("Updating Prometheus ServiceMonitor", "ServiceMonitor.Name", desired.Name)
		found.Spec = desired.Spec
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}
	// ServiceMonitor already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: Prometheus ServiceMonitor already exists", "ServiceMonitor.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deletePrometheusResources(namespace string) error {
	if used, err := r.checkPrometheusUsed(); used == false {
		return err
	}

	rule := &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), rule); err != nil && !errors.IsNotFound(err) {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), role); err != nil && !errors.IsNotFound(err) {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), roleBinding); err != nil && !errors.IsNotFound(err) {
		return err
	}

	monitor := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      monitorName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), monitor); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func createPrometheusRule(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *promv1.PrometheusRule {
	return &promv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
			Labels: map[string]string{
				PrometheusLabelKey: PrometheusLabelValue,
			},
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name: "hpp.rules",
					Rules: []promv1.Rule{
						generateRecordRule(
							"hpp_num_up_operators",
							fmt.Sprintf("sum(up{namespace='%s', pod=~'hostpath-provisioner-operator-.*'} or vector(0))", namespace),
						),
						generateAlertRule(
							"HppOperatorDown",
							"hpp_num_up_operators == 0",
							"5m",
							map[string]string{
								"summary": "Hostpath Provisioner operator is down",
							},
							map[string]string{
								"severity": "warning",
							},
						),
					},
				},
			},
		},
	}
}

func createPrometheusRole(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				PrometheusLabelKey: PrometheusLabelValue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{
					"get", "list", "watch",
				},
			},
		},
	}
}

func createPrometheusRoleBinding(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *rbacv1.RoleBinding {
	monitoringNamespace := getMonitoringNamespace()

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				PrometheusLabelKey: PrometheusLabelValue,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     rbacName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: monitoringNamespace,
				Name:      "prometheus-k8s",
			},
		},
	}
}

func createPrometheusServiceMonitor(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *promv1.ServiceMonitor {
	return &promv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      monitorName,
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "",
				PrometheusLabelKey:                PrometheusLabelValue,
			},
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					PrometheusLabelKey: PrometheusLabelValue,
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:   "metrics",
					Scheme: "http",
					TLSConfig: &promv1.TLSConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
	}
}

func getMonitoringNamespace() string {
	if ns := os.Getenv("MONITORING_NAMESPACE"); ns != "" {
		return ns
	}

	return defaultMonitoringNs
}

func generateAlertRule(alert, expr, duration string, annotations, labels map[string]string) promv1.Rule {
	return promv1.Rule{
		Alert:       alert,
		Expr:        intstr.FromString(expr),
		For:         duration,
		Annotations: annotations,
		Labels:      labels,
	}
}

func generateRecordRule(record, expr string) promv1.Rule {
	return promv1.Rule{
		Record: record,
		Expr:   intstr.FromString(expr),
	}
}

func (r *ReconcileHostPathProvisioner) checkPrometheusUsed() (bool, error) {
	// Check if we are using prometheus, if not return false.
	listObj := &promv1.PrometheusRuleList{}
	if err := r.client.List(context.TODO(), listObj); err != nil {
		if meta.IsNoMatchError(err) {
			// prometheus not deployed
			return false, nil
		}
		return false, err
	}
	return true, nil
}
