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
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	secv1 "github.com/openshift/api/security/v1"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log                = logf.Log.WithName("controller_hostpathprovisioner")
	watchNamespaceFunc = k8sutil.GetWatchNamespace
	readyGauge         = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kubevirt_hpp_cr_ready",
			Help: "HPP CR Ready",
		})
)

func init() {
	metrics.Registry = prometheus.NewRegistry()
	metrics.Registry.MustRegister(readyGauge)
	// 0 is our 'something bad is going on' value for alert to start firing, so can't default to that
	readyGauge.Set(-1)
}

const (
	snapshotFeatureGate = "Snapshotting"
	hppFinalizer        = "finalizer.delete.hostpath-provisioner"
)

func isErrCacheNotStarted(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*cache.ErrCacheNotStarted)
	return ok
}

// Add creates a new HostPathProvisioner Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	mgrScheme := mgr.GetScheme()
	if err := hostpathprovisionerv1.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	return &ReconcileHostPathProvisioner{
		client:   mgr.GetClient(),
		scheme:   mgrScheme,
		recorder: mgr.GetEventRecorderFor("operator-controller"),
		Log:      log,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("hostpathprovisioner-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// mapFn will be used to map reconcile requests to the HPP for resources that don't have an ownerRef
	mapFn := handler.MapFunc(func(o client.Object) []reconcile.Request {
		if val, ok := o.GetLabels()["k8s-app"]; ok && val == MultiPurposeHostPathProvisionerName {
			hppList, err := getHppList(mgr.GetClient())
			if err != nil {
				log.Error(err, "Error getting HPPs")
				return nil
			}
			if size := len(hppList.Items); size != 1 {
				log.Info("There should be exactly one HPP instance")
				return nil
			}

			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: hppList.Items[0].Name},
			}}
		}
		return nil
	})

	// Watch for changes to primary resource HostPathProvisioner
	err = c.Watch(&source.Kind{Type: &hostpathprovisionerv1.HostPathProvisioner{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbacv1.RoleBinding{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbacv1.Role{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &storagev1.CSIDriver{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &rbacv1.ClusterRoleBinding{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &rbacv1.ClusterRole{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
		return err
	}

	if used, err := r.(*ReconcileHostPathProvisioner).checkSCCUsed(); used || isErrCacheNotStarted(err) {
		if err := c.Watch(&source.Kind{Type: &secv1.SecurityContextConstraints{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("Not watching SecurityContextConstraints")
				return nil
			}
			return err
		}
	}

	if used, err := r.(*ReconcileHostPathProvisioner).checkPrometheusUsed(); used || isErrCacheNotStarted(err) {
		if err := c.Watch(&source.Kind{Type: &promv1.PrometheusRule{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("Not watching PrometheusRules")
				return nil
			}
			return err
		}
		if err := c.Watch(&source.Kind{Type: &promv1.ServiceMonitor{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("Not watching ServiceMonitors")
				return nil
			}
			return err
		}
		if err := c.Watch(&source.Kind{Type: &rbacv1.Role{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			return err
		}
		if err := c.Watch(&source.Kind{Type: &rbacv1.RoleBinding{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			return err
		}
		if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(mapFn)); err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileHostPathProvisioner implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHostPathProvisioner{}

// ReconcileHostPathProvisioner reconciles a HostPathProvisioner object
type ReconcileHostPathProvisioner struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	Log      logr.Logger
}

// Reconcile reads that state of the cluster for a HostPathProvisioner object and makes changes based on the state read
// and what is in the HostPathProvisioner.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHostPathProvisioner) Reconcile(context context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(3).Info("Reconciling HostPathProvisioner")

	// Checks that only a single HPP instance exists
	hppList, err := getHppList(r.client)
	if err != nil {
		reqLogger.Error(err, "Error getting HPPs")
		return reconcile.Result{}, err
	}
	if size := len(hppList.Items); size > 1 {
		err := fmt.Errorf("there should be a single hostpath provisioner, %d items found", size)
		reqLogger.Error(err, "Multiple HPPs detected")
		return reconcile.Result{}, err
	}

	versionString, err := version.VersionStringFunc()
	if err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the HostPathProvisioner instance
	cr := &hostpathprovisionerv1.HostPathProvisioner{}
	err = r.client.Get(context, request.NamespacedName, cr)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	reqLogger.Info("Reconciling CSI and legacy controller plugin")

	// Ready metric so we can alert whenever we are not ready for a while
	if IsHppAvailable(cr) {
		readyGauge.Set(1)
	} else if !IsHppProgressing(cr) {
		// Not an issue if progress is still ongoing
		readyGauge.Set(0)
	}

	namespace, err := watchNamespaceFunc()
	if err != nil {
		MarkCrFailed(cr, watchNameSpace, err.Error())
		r.recorder.Event(cr, corev1.EventTypeWarning, watchNameSpace, err.Error())
		err2 := r.client.Update(context, cr)
		if err2 != nil {
			reqLogger.Error(err2, "Unable to update CR to failed state")
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if cr.GetDeletionTimestamp() != nil {
		if err := r.cleanDeployments(reqLogger, cr, namespace); err != nil {
			return reconcile.Result{}, err
		}
		deployments, err := r.currentStoragePoolDeployments(reqLogger, cr, namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("Number of deployments still active", "count", len(deployments))
		cleanupFinished, err := r.hasCleanUpFinished()
		if err != nil {
			return reconcile.Result{}, err
		}
		if len(deployments) == 0 && cleanupFinished {
			if err := r.removeCleanUpJobs(reqLogger); err != nil {
				return reconcile.Result{}, err
			}
		} else {
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}
		reqLogger.Info("Deleting SecurityContextConstraint", "SecurityContextConstraints", MultiPurposeHostPathProvisionerName)
		if err := r.deleteSCC(MultiPurposeHostPathProvisionerName); err != nil {
			reqLogger.Error(err, "Unable to delete SecurityContextConstraints")
			// TODO, should we return and in essence keep retrying, and thus never be able to delete the CR if deleting the SCC fails, or
			// should be not return and allow the CR to be deleted but without deleting the SCC if that fails.
			return reconcile.Result{}, err
		}
		if err := r.deleteSCC(fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)); err != nil {
			reqLogger.Error(err, "Unable to delete CSI SecurityContextConstraints")
			// TODO, should we return and in essence keep retrying, and thus never be able to delete the CR if deleting the SCC fails, or
			// should be not return and allow the CR to be deleted but without deleting the SCC if that fails.
			return reconcile.Result{}, err
		}
		if err := r.deletePrometheusResources(namespace); err != nil {
			reqLogger.Error(err, "Unable to delete Prometheus Infra (PrometheusRule, ServiceMonitor, RBAC)")
			return reconcile.Result{}, err
		}
		if res, err := r.deleteAllRbac(reqLogger, namespace); err != nil {
			return res, err
		}
		reqLogger.Info("Deleting CSIDriver", "CSIDriver", MultiPurposeHostPathProvisionerName)
		if err := r.deleteCSIDriver(); err != nil {
			reqLogger.Error(err, "Unable to delete CSIDriver")
			return reconcile.Result{}, err
		}
		RemoveFinalizer(cr, hppFinalizer)

		// Update CR
		err = r.client.Update(context, cr)
		if err != nil {
			reqLogger.Error(err, "Unable to remove finalizer from CR")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if err := r.addFinalizer(reqLogger, cr); err != nil {
		return reconcile.Result{}, err
	}

	cr.Status.OperatorVersion = versionString
	cr.Status.TargetVersion = versionString
	canUpgrade, err := canUpgrade(cr.Status.ObservedVersion, versionString)
	if err != nil {
		// Downgrading not supported
		return reconcile.Result{}, err
	}
	if r.isDeploying(cr) {
		//New install, mark deploying.
		MarkCrDeploying(cr, deployStarted, deployStartedMessage)
		r.recorder.Event(cr, corev1.EventTypeNormal, deployStarted, deployStartedMessage)
		err = r.client.Update(context, cr)
		if err != nil {
			reqLogger.Info("Marked deploying failed", "Error", err.Error())
			// Error updating the object - requeue the request.
			return reconcile.Result{}, err
		}
		reqLogger.Info("Started deploying")
	}

	if canUpgrade && r.isUpgrading(cr) {
		MarkCrUpgradeHealingDegraded(cr, upgradeStarted, fmt.Sprintf("Started upgrade to version %s", cr.Status.TargetVersion))
		r.recorder.Event(cr, corev1.EventTypeWarning, upgradeStarted, fmt.Sprintf("Started upgrade to version %s", cr.Status.TargetVersion))
		// Mark Observed version to blank, so we get to the reconcile upgrade section.
		err = r.client.Update(context, cr)
		if err != nil {
			// Error updating the object - requeue the request.
			return reconcile.Result{}, err
		}
		reqLogger.Info("Started upgrading")
	}

	res, err := r.reconcileUpdate(reqLogger, request, cr, namespace)
	if err == nil {
		return r.reconcileStatus(context, reqLogger, cr, namespace, versionString)
	}
	MarkCrFailedHealing(cr, reconcileFailed, fmt.Sprintf("Unable to successfully reconcile: %v", err))
	r.recorder.Event(cr, corev1.EventTypeWarning, reconcileFailed, fmt.Sprintf("Unable to successfully reconcile: %v", err))
	return res, err
}

func (r *ReconcileHostPathProvisioner) reconcileStatus(context context.Context, reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace, versionString string) (reconcile.Result, error) {
	// Check if all requested pods are available.
	degraded, err := r.checkDegraded(reqLogger, cr, namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := r.reconcileStoragePoolStatus(reqLogger, cr, namespace); err != nil {
		return reconcile.Result{}, err
	}
	if !degraded && cr.Status.ObservedVersion != versionString {
		cr.Status.ObservedVersion = versionString
	}
	err = r.client.Update(context, cr)
	if err != nil {
		// Error updating the object - requeue the request.
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) deleteAllRbac(reqLogger logr.Logger, namespace string) (reconcile.Result, error) {
	for _, name := range []string{ProvisionerServiceAccountName, ProvisionerServiceAccountNameCsi, healthCheckName, MultiPurposeHostPathProvisionerName} {
		reqLogger.Info("Deleting ClusterRoleBinding", "ClusterRoleBinding", name)
		if err := r.deleteClusterRoleBindingObject(name); err != nil {
			reqLogger.Error(err, "Unable to delete ClusterRoleBinding")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deleting ClusterRole", "ClusterRole", name)
		if err := r.deleteClusterRoleObject(name); err != nil {
			reqLogger.Error(err, "Unable to delete ClusterRole")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deleting RoleBinding", "ClusterRoleBinding", name)
		if err := r.deleteRoleBindingObject(name, namespace); err != nil {
			reqLogger.Error(err, "Unable to delete RoleBinding")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deleting Role", "ClusterRole", name)
		if err := r.deleteRoleObject(name, namespace); err != nil {
			reqLogger.Error(err, "Unable to delete Role")
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func canUpgrade(current, target string) (bool, error) {
	if current == "" {
		// Can't upgrade if no current is set
		return false, nil
	}

	if target == current {
		return false, nil
	}

	result := true
	// semver doesn't like the 'v' prefix
	targetSemver, errTarget := version.GetVersionFromString(target)
	currentSemver, errCurrent := version.GetVersionFromString(current)

	if errTarget == nil && errCurrent == nil {
		if targetSemver.Compare(*currentSemver) < 0 {
			err := fmt.Errorf("operator downgraded from %s to %s, will not reconcile", currentSemver.String(), targetSemver.String())
			return false, err
		} else if targetSemver.Compare(*currentSemver) == 0 {
			result = false
		}
	}
	return result, nil
}

func (r *ReconcileHostPathProvisioner) reconcileUpdate(reqLogger logr.Logger, request reconcile.Request, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Reconcile the objects this operator manages.
	res, err := r.reconcileDaemonSet(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create DaemonSet")
		return res, err
	}
	// Reconcile storage pools
	res, err = r.reconcileStoragePools(reqLogger, cr, namespace)
	if err != nil {
		reqLogger.Error(err, "unable to configure storage pools")
		return res, err
	}
	res, err = r.reconcileServiceAccount(reqLogger, cr, namespace)
	if err != nil {
		reqLogger.Error(err, "unable to create ServiceAccount")
		return res, err
	}
	res, err = r.reconcileClusterRole(reqLogger, cr, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create ClusterRole")
		return res, err
	}
	res, err = r.reconcileClusterRoleBinding(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create ClusterRoleBinding")
		return res, err
	}
	res, err = r.reconcileRole(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create Role")
		return res, err
	}
	res, err = r.reconcileRoleBinding(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create RoleBinding")
		return res, err
	}
	res, err = r.reconcileCSIDriver(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create CSIDriver")
		return res, err
	}
	res, err = r.reconcileSecurityContextConstraints(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create SecurityContextConstraints")
		return res, err
	}
	res, err = r.reconcilePrometheusInfra(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "unable to create Prometheus Infra (PrometheusRule, ServiceMonitor, RBAC)")
		return res, err
	}
	daemonSet := &appsv1.DaemonSet{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: MultiPurposeHostPathProvisionerName, Namespace: namespace}, daemonSet); err != nil {
		return reconcile.Result{}, err
	}
	daemonSetCsi := &appsv1.DaemonSet{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName), Namespace: namespace}, daemonSetCsi); err != nil {
		return reconcile.Result{}, err
	}
	if checkDaemonSetReady(daemonSet) && checkDaemonSetReady(daemonSetCsi) {
		MarkCrHealthyMessage(cr, "Complete", "Application Available")
		r.recorder.Event(cr, corev1.EventTypeNormal, provisionerHealthy, provisionerHealthyMessage)
	}
	return res, nil
}

func (r *ReconcileHostPathProvisioner) checkDegraded(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (bool, error) {
	degraded := false

	daemonSet := &appsv1.DaemonSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: MultiPurposeHostPathProvisionerName, Namespace: namespace}, daemonSet)
	if err != nil {
		return true, err
	}
	daemonSetCsi := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName), Namespace: namespace}, daemonSetCsi)
	if err != nil {
		return true, err
	}

	if !(checkDaemonSetReady(daemonSet) && checkDaemonSetReady(daemonSetCsi)) {
		degraded = true
	}

	logger.V(3).Info("Degraded check", "Degraded", degraded)

	// If deployed and degraded, mark degraded, otherwise we are still deploying or not degraded.
	if degraded && !r.isDeploying(cr) {
		conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionTrue,
		})
	} else {
		conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionFalse,
		})
	}

	logger.V(3).Info("Finished degraded check", "conditions", cr.Status.Conditions)
	return degraded, nil
}

func checkDaemonSetReady(daemonSet *appsv1.DaemonSet) bool {
	return checkApplicationAvailable(daemonSet) && daemonSet.Status.NumberReady >= daemonSet.Status.DesiredNumberScheduled
}

func checkApplicationAvailable(daemonSet *appsv1.DaemonSet) bool {
	return daemonSet.Status.NumberReady > 0
}

func (r *ReconcileHostPathProvisioner) addFinalizer(reqLogger logr.Logger, obj client.Object) error {
	if obj.GetDeletionTimestamp() == nil {
		reqLogger.Info("Adding deletion Finalizer")
		AddFinalizer(obj, hppFinalizer)

		// Update CR
		err := r.client.Update(context.TODO(), obj)
		if err != nil {
			reqLogger.Error(err, "Failed to update cr with finalizer")
			return err
		}
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) isFeatureGateEnabled(feature string, cr *hostpathprovisionerv1.HostPathProvisioner) bool {
	for _, featuregate := range cr.Spec.FeatureGates {
		if featuregate == feature {
			return true
		}
	}
	return false
}

// This function returns the list of HPP instances in the cluster and an error otherwise
func getHppList(c client.Client) (*hostpathprovisionerv1.HostPathProvisionerList, error) {
	hppList := &hostpathprovisionerv1.HostPathProvisionerList{}

	if err := c.List(context.TODO(), hppList, &client.ListOptions{}); err != nil {
		return nil, err
	}

	return hppList, nil
}

// AddFinalizer adds a finalizer to a resource
func AddFinalizer(obj metav1.Object, name string) {
	if HasFinalizer(obj, name) {
		return
	}

	obj.SetFinalizers(append(obj.GetFinalizers(), name))
}

// RemoveFinalizer removes a finalizer from a resource
func RemoveFinalizer(obj metav1.Object, name string) {
	if !HasFinalizer(obj, name) {
		return
	}

	var finalizers []string
	for _, f := range obj.GetFinalizers() {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}

	obj.SetFinalizers(finalizers)
}

// HasFinalizer returns true if a resource has a specific finalizer
func HasFinalizer(object metav1.Object, value string) bool {
	for _, f := range object.GetFinalizers() {
		if f == value {
			return true
		}
	}
	return false
}
