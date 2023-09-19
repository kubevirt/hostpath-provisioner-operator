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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

const (
	ruleName                  = "prometheus-hpp-rules"
	rbacName                  = "hostpath-provisioner-monitoring"
	monitorName               = "service-monitor-hpp"
	defaultMonitoringNs       = "monitoring"
	defaultRunbookURLTemplate = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv     = "RUNBOOK_URL_TEMPLATE"
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
	partOfAlertLabelKey       = "kubernetes_operator_part_of"
	partOfAlertLabelValue     = "kubevirt"
	componentAlertLabelKey    = "kubernetes_operator_component"
	componentAlertLabelValue  = "hostpath-provisioner-operator"
)

func (r *ReconcileHostPathProvisioner) reconcilePrometheusInfra(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	if used, err := r.checkPrometheusUsed(); err != nil {
		return reconcile.Result{}, err
	} else if used == false {
		return reconcile.Result{}, nil
	}
	if res, err := r.reconcilePrometheusResource(reqLogger, cr, createPrometheusRule(namespace), createPrometheusRule(namespace)); err != nil {
		return res, err
	}
	if res, err := r.reconcilePrometheusResource(reqLogger, cr, createPrometheusRole(namespace), createPrometheusRole(namespace)); err != nil {
		return res, err
	}
	if res, err := r.reconcilePrometheusResource(reqLogger, cr, createPrometheusRoleBinding(namespace), createPrometheusRoleBinding(namespace)); err != nil {
		return res, err
	}
	if res, err := r.reconcilePrometheusResource(reqLogger, cr, createPrometheusService(namespace), createPrometheusService(namespace)); err != nil {
		return res, err
	}
	return r.reconcilePrometheusResource(reqLogger, cr, createPrometheusServiceMonitor(namespace), createPrometheusServiceMonitor(namespace))
}

func (r *ReconcileHostPathProvisioner) reconcilePrometheusResource(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, desired, found client.Object) (reconcile.Result, error) {
	// Define a new PrometheusRule object
	setLastAppliedConfiguration(desired)
	// Check if this PrometheusRule already exists
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(found), found)
	if err != nil && k8serrors.IsNotFound(err) {
		reqLogger.Info("Creating a new PrometheusResource", "Name", found.GetName())
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.GetName(), err))
			return reconcile.Result{}, err
		}
		// PrometheusRule created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.GetName()))
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	// create merged PrometheusRule from found and desired.
	merged, err := mergeObject(desired, found)
	if err != nil {
		return reconcile.Result{}, err
	}

	// PrometheusRule already exists, check if we need to update.
	if !reflect.DeepEqual(currentRuntimeObjCopy, merged) {
		logJSONDiff(reqLogger, currentRuntimeObjCopy, merged)
		// Current is different from desired, update.
		reqLogger.Info("Updating PrometheusResource", "Name", desired.GetName())
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.GetName(), err))
			return reconcile.Result{}, err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.GetName()))
		return reconcile.Result{}, nil
	}
	// PrometheusResource already exists and matches the desired state - don't requeue
	reqLogger.Info("Skip reconcile: PrometheusResource already exists", "Name", found.GetName())
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
	if err := r.client.Delete(context.TODO(), rule); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), role); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), roleBinding); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	monitor := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      monitorName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), monitor); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrometheusServiceName,
			Namespace: namespace,
		},
	}
	if err := r.client.Delete(context.TODO(), service); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

// RecordRulesDesc represent HPP Prometheus Record Rules
type RecordRulesDesc struct {
	Name        string
	Expr        string
	Description string
	Type        string
}

// GetRecordRulesDesc returns HPPgst Prometheus Record Rules
func GetRecordRulesDesc(namespace string) []RecordRulesDesc {
	return []RecordRulesDesc{
		{
			"kubevirt_hpp_operator_up",
			fmt.Sprintf("sum(up{namespace='%s', pod=~'hostpath-provisioner-operator-.*'} or vector(0))", namespace),
			"The number of running hostpath-provisioner-operator pods",
			"Gauge",
		},
	}
}

func getRecordRules(namespace string) []promv1.Rule {
	var recordRules []promv1.Rule

	for _, rrd := range GetRecordRulesDesc(namespace) {
		recordRules = append(recordRules, generateRecordRule(rrd.Name, rrd.Expr))
	}

	return recordRules
}

func getAlertRules(runbookURLTemplate string) []promv1.Rule {
	return []promv1.Rule{
		generateAlertRule(
			"HPPOperatorDown",
			"kubevirt_hpp_operator_up == 0",
			"5m",
			map[string]string{
				"summary":     "Hostpath Provisioner operator is down",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "HPPOperatorDown"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"HPPNotReady",
			"kubevirt_hpp_cr_ready == 0",
			"5m",
			map[string]string{
				"summary":     "Hostpath Provisioner is not available to use",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "HPPNotReady"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"HPPSharingPoolPathWithOS",
			"kubevirt_hpp_pool_path_shared_with_os == 1",
			"1m",
			map[string]string{
				"summary":     "HPP pool path sharing a filesystem with OS, fix to prevent HPP PVs from causing disk pressure and affecting node operation",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "HPPSharingPoolPathWithOS"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
	}
}

func createPrometheusRule(namespace string) *promv1.PrometheusRule {
	labels := getRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue

	runbookURLTemplate := getRunbookURLTemplate()

	return &promv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name:  "hpp.rules",
					Rules: append(getRecordRules(namespace), getAlertRules(runbookURLTemplate)...),
				},
			},
		},
	}
}

func createPrometheusRole(namespace string) *rbacv1.Role {
	labels := getRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels:    labels,
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

func createPrometheusRoleBinding(namespace string) *rbacv1.RoleBinding {
	monitoringNamespace := getMonitoringNamespace()
	labels := getRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels:    labels,
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

func createPrometheusServiceMonitor(namespace string) *promv1.ServiceMonitor {
	labels := getRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue
	labels["openshift.io/cluster-monitoring"] = ""

	return &promv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      monitorName,
			Labels:    labels,
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

func createPrometheusService(namespace string) *corev1.Service {
	labels := getRecommendedLabels()
	labels[PrometheusLabelKey] = PrometheusLabelValue

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      PrometheusServiceName,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{PrometheusLabelKey: PrometheusLabelValue},
			Ports: []corev1.ServicePort{
				{
					Name: "metrics",
					Port: 8080,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "metrics",
					},
					Protocol: corev1.ProtocolTCP,
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
		if meta.IsNoMatchError(err) || strings.Contains(err.Error(), "failed to find API group") {
			// prometheus not deployed
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getRunbookURLTemplate() string {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		panic(errors.New("runbook URL template must have exactly 1 %s substring"))
	}

	return runbookURLTemplate
}
