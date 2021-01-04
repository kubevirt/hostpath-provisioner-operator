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
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_hostpathprovisioner")
var watchNamespaceFunc = k8sutil.GetWatchNamespace

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

	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbacv1.ClusterRoleBinding{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &rbacv1.ClusterRole{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
	})
	if err != nil {
		return err
	}

	if used, _ := r.(*ReconcileHostPathProvisioner).checkSCCUsed(); used == true {
		err = c.Watch(&source.Kind{Type: &secv1.SecurityContextConstraints{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &hostpathprovisionerv1.HostPathProvisioner{},
		})
		if err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("Not watching SecurityContextConstraints")
				return nil
			}
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
func (r *ReconcileHostPathProvisioner) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(3).Info("Reconciling HostPathProvisioner")
	versionString, err := version.VersionStringFunc()
	if err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the HostPathProvisioner instance
	cr := &hostpathprovisionerv1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), request.NamespacedName, cr)
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

	isMarkedToBeDeleted := cr.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		reqLogger.Info("Deleting SecurityContextConstraint", "SecurityContextConstraints", cr.Name)
		if err := r.deleteSCC(cr); err != nil {
			reqLogger.Error(err, "Unable to delete SecurityContextConstraints")
			// TODO, should we return and in essence keep retrying, and thus never be able to delete the CR if deleting the SCC fails, or
			// should be not return and allow the CR to be deleted but without deleting the SCC if that fails.
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deleting ClusterRoleBinding", "ClusterRoleBinding", cr.Name)
		if err := r.deleteClusterRoleBindingObject(cr); err != nil {
			reqLogger.Error(err, "Unable to delete ClusterRoleBinding")
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deleting ClusterRole", "ClusterRole", cr.Name)
		if err := r.deleteClusterRoleObject(cr); err != nil {
			reqLogger.Error(err, "Unable to delete ClusterRole")
			return reconcile.Result{}, err
		}

		cr.SetFinalizers(nil)

		// Update CR
		err = r.client.Update(context.TODO(), cr)
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

	namespace, err := watchNamespaceFunc()
	if err != nil {
		MarkCrFailed(cr, watchNameSpace, err.Error())
		r.recorder.Event(cr, corev1.EventTypeWarning, watchNameSpace, err.Error())
		err2 := r.client.Update(context.TODO(), cr)
		if err2 != nil {
			reqLogger.Error(err2, "Unable to update CR to failed state")
		}
		// Error reading the object - requeue the request.
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
		err = r.client.Update(context.TODO(), cr)
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
		err = r.client.Update(context.TODO(), cr)
		if err != nil {
			// Error updating the object - requeue the request.
			return reconcile.Result{}, err
		}
		reqLogger.Info("Started upgrading")
	}

	res, err := r.reconcileUpdate(reqLogger, request, cr, namespace)
	if err == nil {
		// Check if all requested pods are available.
		degraded, err := r.checkDegraded(reqLogger, cr, namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !degraded && cr.Status.ObservedVersion != versionString {
			cr.Status.ObservedVersion = versionString
			err = r.client.Update(context.TODO(), cr)
			if err != nil {
				// Error updating the object - requeue the request.
				return reconcile.Result{}, err
			}
		}
	} else {
		MarkCrFailedHealing(cr, reconcileFailed, fmt.Sprintf("Unable to successfully reconcile: %v", err))
		r.recorder.Event(cr, corev1.EventTypeWarning, reconcileFailed, fmt.Sprintf("Unable to successfully reconcile: %v", err))
	}
	return res, nil
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
		reqLogger.Error(err, "Unable to create DaemonSet")
		return reconcile.Result{}, err
	}
	res, err = r.reconcileServiceAccount(cr, namespace)
	if err != nil {
		reqLogger.Error(err, "Unable to create ServiceAccount")
		return reconcile.Result{}, err
	}
	res, err = r.reconcileClusterRole(reqLogger, cr, r.recorder)
	if err != nil {
		reqLogger.Error(err, "Unable to create ClusterRole")
		return reconcile.Result{}, err
	}
	res, err = r.reconcileClusterRoleBinding(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "Unable to create ClusterRoleBinding")
		return reconcile.Result{}, err
	}
	res, err = r.reconcileSecurityContextConstraints(reqLogger, cr, namespace, r.recorder)
	if err != nil {
		reqLogger.Error(err, "Unable to create SecurityContextConstraints")
		return reconcile.Result{}, err
	}
	daemonSet := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: namespace}, daemonSet)
	if err != nil {
		return reconcile.Result{}, err
	}
	if checkApplicationAvailable(daemonSet) {
		if !IsCrHealthy(cr) {
			r.recorder.Event(cr, corev1.EventTypeNormal, provisionerHealthy, provisionerHealthyMessage)
		}
		MarkCrHealthyMessage(cr, "Complete", "Application Available")
	}
	return res, nil
}

func (r *ReconcileHostPathProvisioner) checkDegraded(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (bool, error) {
	degraded := false

	daemonSet := &appsv1.DaemonSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: namespace}, daemonSet)
	if err != nil {
		return true, err
	}

	if !checkDaemonSetReady(daemonSet) {
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

func (r *ReconcileHostPathProvisioner) reconcileServiceAccount(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	// Define a new Service Account object
	desired := createServiceAccountObject(cr, namespace)
	setLastAppliedConfiguration(desired)

	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, desired, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this ServiceAccount already exists
	found := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new Service Account", "ServiceAccount.Namespace", desired.Namespace, "ServiceAccount.Name", desired.Name)
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceStart, fmt.Sprintf(createMessageStart, desired, desired.Name))
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}

		// Service Account created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
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
		log.Info("Updating Service Account", "ServiceAccount.Name", desired.Name)
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceStart, fmt.Sprintf(updateMessageStart, desired, desired.Name))
		err = r.client.Update(context.TODO(), merged)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.Name, err))
			return reconcile.Result{}, err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.Name))
		return reconcile.Result{}, nil
	}

	// Service Account already exists and matches desired - don't requeue
	log.V(3).Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)
	return reconcile.Result{}, nil
}

// createServiceAccount returns a new Service Account object in the same namespace as the cr.
func createServiceAccountObject(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) *corev1.ServiceAccount {
	labels := map[string]string{
		"k8s-app": cr.Name,
	}
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-admin",
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func (r *ReconcileHostPathProvisioner) addFinalizer(reqLogger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner) error {
	if len(cr.GetFinalizers()) < 1 && cr.GetDeletionTimestamp() == nil {
		reqLogger.Info("Adding deletion Finalizer")
		cr.SetFinalizers([]string{"finalizer.delete.hostpath-provisioner"})

		// Update CR
		err := r.client.Update(context.TODO(), cr)
		if err != nil {
			reqLogger.Error(err, "Failed to update cr with finalizer")
			return err
		}
	}
	return nil
}
