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
	"strings"

	secv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1alpha1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	conditions "github.com/openshift/custom-resource-status/conditions/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	versionString = "1.0.1"
)

var _ = Describe("Controller reconcile loop", func() {
	var (
		cr *v1alpha1.HostPathProvisioner
		cl client.Client
		r  *ReconcileHostPathProvisioner
	)

	BeforeEach(func() {
		watchNamespaceFunc = func() (string, error) {
			return "test-namespace", nil
		}
		version.VersionStringFunc = func() *string {
			value := versionString
			return &value
		}

		cr = &v1alpha1.HostPathProvisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
			Spec: v1alpha1.HostPathProvisionerSpec{
				ImageRegistry:   "test-registry",
				ImagePullPolicy: corev1.PullAlways,
				ImageTag:        "canary",
				PathConfig: v1alpha1.PathConfig{
					Path:            "/tmp/test",
					UseNamingPrefix: "false",
				},
			},
		}
	})

	It("Should create new if none exist", func() {
		createDeployedCr(cr)
	})

	It("Should fix a changed daemonSet", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the daemonSet to something not desired.
		ds := &appsv1.DaemonSet{}
		err := cl.Get(context.TODO(), req.NamespacedName, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Spec.Template.Spec.Volumes[0].Name = "invalid"
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())
		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), req.NamespacedName, ds)
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal("invalid"))

		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Check the daemonSet value, make sure it changed back.
		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), req.NamespacedName, ds)
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal("pv-volume"))
	})

	It("Should fix a changed storage class (excluding annotations)", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "test-name",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the storage class to something not desired.
		sc := &storagev1.StorageClass{}
		err := cl.Get(context.TODO(), req.NamespacedName, sc)
		Expect(err).NotTo(HaveOccurred())
		recycle := corev1.PersistentVolumeReclaimRecycle
		sc.ReclaimPolicy = &recycle
		err = cl.Update(context.TODO(), sc)
		Expect(err).NotTo(HaveOccurred())
		sc = &storagev1.StorageClass{}
		err = cl.Get(context.TODO(), req.NamespacedName, sc)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.ReclaimPolicy).To(Equal(&recycle))

		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Check the storage class value, make sure it changed back.
		sc = &storagev1.StorageClass{}
		err = cl.Get(context.TODO(), req.NamespacedName, sc)
		Expect(err).NotTo(HaveOccurred())
		delete := corev1.PersistentVolumeReclaimDelete
		Expect(sc.ReclaimPolicy).To(Equal(&delete))

		annotations := sc.GetObjectMeta().GetAnnotations()
		annotations["storageclass.kubernetes.io/is-default-class"] = "true"
		sc.GetObjectMeta().SetAnnotations(annotations)
		err = cl.Update(context.TODO(), sc)
		Expect(err).NotTo(HaveOccurred())
		sc = &storagev1.StorageClass{}
		err = cl.Get(context.TODO(), req.NamespacedName, sc)
		Expect(err).NotTo(HaveOccurred())
		if val, ok := sc.GetObjectMeta().GetAnnotations()["storageclass.kubernetes.io/is-default-class"]; ok {
			Expect(val).To(Equal("true"))
		}
		// Run the reconcile loop
		res, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		sc = &storagev1.StorageClass{}
		err = cl.Get(context.TODO(), req.NamespacedName, sc)
		Expect(err).NotTo(HaveOccurred())
		if val, ok := sc.GetObjectMeta().GetAnnotations()["storageclass.kubernetes.io/is-default-class"]; ok {
			Expect(val).To(Equal("true"))
		} else {
			Fail("Annotations changed, default storage class removed")
		}
	})

	It("Should fix a changed service account", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		saNN := types.NamespacedName{
			Name:      "test-name-admin",
			Namespace: "test-namespace",
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the service account to something not desired.
		sa := &corev1.ServiceAccount{}
		err := cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		sa.ObjectMeta.Labels["k8s-app"] = "invalid"
		err = cl.Update(context.TODO(), sa)
		Expect(err).NotTo(HaveOccurred())
		sa = &corev1.ServiceAccount{}
		err = cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.ObjectMeta.Labels["k8s-app"]).To(Equal("invalid"))
		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify the label has been changed back.
		sa = &corev1.ServiceAccount{}
		err = cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.ObjectMeta.Labels["k8s-app"]).To(Equal("test-name"))
	})

	It("Should fix a changed ClusterRole", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		croleNN := types.NamespacedName{
			Name: "test-name",
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the ClusterRole to something not desired.
		crole := &rbacv1.ClusterRole{}
		err := cl.Get(context.TODO(), croleNN, crole)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crole.Rules[1].Verbs)).To(Equal(4))
		// Add delete to persistentvolumeclaims rule
		crole.Rules[1].Verbs = append(crole.Rules[1].Verbs, "delete")
		err = cl.Update(context.TODO(), crole)
		Expect(err).NotTo(HaveOccurred())
		crole = &rbacv1.ClusterRole{}
		err = cl.Get(context.TODO(), croleNN, crole)
		Expect(err).NotTo(HaveOccurred())
		// Verify the extra ability is there.
		Expect(len(crole.Rules[1].Verbs)).To(Equal(5))
		Expect(crole.Rules[1].Verbs[4]).To(Equal("delete"))
		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify its gone now
		err = cl.Get(context.TODO(), croleNN, crole)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crole.Rules[1].Verbs)).To(Equal(4))
	})

	It("Should fix a changed ClusterRoleBinding", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		crbNN := types.NamespacedName{
			Name: "test-name",
		}
		cr, r, cl = createDeployedCr(cr)

		// Now modify the CRB to something not desired.
		crb := &rbacv1.ClusterRoleBinding{}
		err := cl.Get(context.TODO(), crbNN, crb)
		Expect(err).NotTo(HaveOccurred())
		crb.Subjects[0].Name = "invalid"
		err = cl.Update(context.TODO(), crb)
		Expect(err).NotTo(HaveOccurred())
		// Verify the name is wrong
		crb = &rbacv1.ClusterRoleBinding{}
		err = cl.Get(context.TODO(), crbNN, crb)
		Expect(err).NotTo(HaveOccurred())
		Expect(crb.Subjects[0].Name).To(Equal("invalid"))
		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify the name is correct again.
		crb = &rbacv1.ClusterRoleBinding{}
		err = cl.Get(context.TODO(), crbNN, crb)
		Expect(err).NotTo(HaveOccurred())
		Expect(crb.Subjects[0].Name).To(Equal("test-name-admin"))
	})

	It("Should fix a changed SecurityContextConstraints", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		sccNN := types.NamespacedName{
			Name: "test-name",
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the SCC to something not desired.
		scc := &secv1.SecurityContextConstraints{}
		err := cl.Get(context.TODO(), sccNN, scc)
		Expect(err).NotTo(HaveOccurred())
		scc.AllowPrivilegedContainer = true
		err = cl.Update(context.TODO(), scc)
		Expect(err).NotTo(HaveOccurred())
		// Verify allowPrivileged is true
		scc = &secv1.SecurityContextConstraints{}
		err = cl.Get(context.TODO(), sccNN, scc)
		Expect(err).NotTo(HaveOccurred())
		Expect(scc.AllowPrivilegedContainer).To(BeTrue())
		// Run the reconcile loop
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify allowPrivileged is false
		scc = &secv1.SecurityContextConstraints{}
		err = cl.Get(context.TODO(), sccNN, scc)
		Expect(err).NotTo(HaveOccurred())
		Expect(scc.AllowPrivilegedContainer).To(BeFalse())
	})

	It("Should requeue if watch namespaces returns error", func() {
		watchNamespaceFunc = func() (string, error) {
			return "", fmt.Errorf("Something is not right, no watch namespace")
		}
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(v1alpha1.SchemeGroupVersion, cr)
		secv1.AddToScheme(s)

		// Create a fake client to mock API calls.
		cl := fake.NewFakeClient(objs...)

		// Create a ReconcileMemcached object with the scheme and fake client.
		r := &ReconcileHostPathProvisioner{
			client: cl,
			scheme: s,
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should requeue if cr cannot be located", func() {
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(v1alpha1.SchemeGroupVersion, cr)
		secv1.AddToScheme(s)

		// Create a fake client to mock API calls.
		cl := fake.NewFakeClient(objs...)

		// Create a ReconcileMemcached object with the scheme and fake client.
		r := &ReconcileHostPathProvisioner{
			client: cl,
			scheme: s,
		}

		// Mock request to simulate Reconcile() being called on an event for a
		// watched resource .
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name2",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should fail if trying to downgrade", func() {
		cr, r, cl = createDeployedCr(cr)
		version.VersionStringFunc = func() *string {
			value := "1.0.0"
			return &value
		}
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		Expect(strings.Contains(err.Error(), "downgraded")).To(BeTrue())
	})

	It("Should update CR status when upgrading", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		version.VersionStringFunc = func() *string {
			value := "1.0.2"
			return &value
		}
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr := &v1alpha1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedCr.Status.OperatorVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.ObservedVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.TargetVersion).To(Equal("1.0.2"))
		// Didn't make daemonset unavailable, so should be fully healthy
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeFalse())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())

		// Upgrade again, but make daemon set unavailable
		version.VersionStringFunc = func() *string {
			value := "1.0.3"
			return &value
		}
		ds := &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), req.NamespacedName, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())

		res, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr = &v1alpha1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedCr.Status.OperatorVersion).To(Equal("1.0.3"))
		Expect(updatedCr.Status.ObservedVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.TargetVersion).To(Equal("1.0.3"))
		// Didn't make daemonset unavailable, so should be fully healthy
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeFalse())
		// It should be degraded
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), req.NamespacedName, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 2
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())

		res, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr = &v1alpha1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedCr.Status.OperatorVersion).To(Equal("1.0.3"))
		Expect(updatedCr.Status.ObservedVersion).To(Equal("1.0.3"))
		Expect(updatedCr.Status.TargetVersion).To(Equal("1.0.3"))
		// Didn't make daemonset unavailable, so should be fully healthy
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeFalse())
		// It should NOT be degraded
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())
	})

	It("Should not requeue when CR is deleted", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		err := cl.Delete(context.TODO(), cr)
		Expect(err).NotTo(HaveOccurred())
		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})
})

// After this has run, the returned cr state should be available, not progressing and not degraded.
func createDeployedCr(cr *v1alpha1.HostPathProvisioner) (*v1alpha1.HostPathProvisioner, *ReconcileHostPathProvisioner, client.Client) {
	objs := []runtime.Object{cr}
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, cr)
	secv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ReconcileHostPathProvisioner{
		client: cl,
		scheme: s,
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
	}
	res, err := r.Reconcile(req)
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Requeue).To(BeFalse())
	updatedCr := &v1alpha1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
	Expect(err).NotTo(HaveOccurred())
	Expect(updatedCr.Status.OperatorVersion).To(Equal(versionString))
	Expect(updatedCr.Status.TargetVersion).To(Equal(versionString))
	Expect(updatedCr.Status.ObservedVersion).To(Equal(""))
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeFalse())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())
	// Verify all the different objects are created.
	verifyCreateDaemonSet(r.client, req.NamespacedName)
	verifyCreateServiceAccount(r.client)
	verifyCreateStorageClass(r.client)
	verifyCreateClusterRole(r.client)
	verifyCreateClusterRoleBinding(r.client)
	verifyCreateSCC(r.client)

	// Now make the daemonSet available, and reconcile again.
	ds := &appsv1.DaemonSet{}
	err = cl.Get(context.TODO(), req.NamespacedName, ds)
	Expect(err).NotTo(HaveOccurred())
	ds.Status.NumberReady = 2
	ds.Status.DesiredNumberScheduled = 2
	err = cl.Update(context.TODO(), ds)
	Expect(err).NotTo(HaveOccurred())

	// daemonSet is ready, now reconcile again. We should have condition changes and observed version should be set.
	res, err = r.Reconcile(req)
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Requeue).To(BeFalse())
	updatedCr = &v1alpha1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
	Expect(err).NotTo(HaveOccurred())
	Expect(updatedCr.Status.OperatorVersion).To(Equal(versionString))
	Expect(updatedCr.Status.TargetVersion).To(Equal(versionString))
	Expect(updatedCr.Status.ObservedVersion).To(Equal(versionString))
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeFalse())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())
	return cr, r, cl
}

// Verify all the proper values are set when creating the daemonset
func verifyCreateDaemonSet(cl client.Client, nn types.NamespacedName) {
	ds := &appsv1.DaemonSet{}
	err := cl.Get(context.TODO(), nn, ds)
	Expect(err).NotTo(HaveOccurred())
	// Check Service Account
	Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal("test-name-admin"))
	// Check container image
	Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal("test-registry/hostpath-provisioner:canary"))
	// Check use naming prefix
	Expect(ds.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("false"))
	// Check directory
	Expect(ds.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal("/tmp/test"))
}

func verifyCreateServiceAccount(cl client.Client) {
	sa := &corev1.ServiceAccount{}
	nn := types.NamespacedName{
		Name:      "test-name-admin",
		Namespace: "test-namespace",
	}
	err := cl.Get(context.TODO(), nn, sa)
	Expect(err).NotTo(HaveOccurred())
	Expect(sa.ObjectMeta.Name).To(Equal("test-name-admin"))
}

func verifyCreateStorageClass(cl client.Client) {
	sc := &storagev1.StorageClass{}
	nn := types.NamespacedName{
		Name: "test-name",
	}
	err := cl.Get(context.TODO(), nn, sc)
	Expect(err).NotTo(HaveOccurred())
	Expect(sc.Provisioner).To(Equal("kubevirt.io/hostpath-provisioner"))
}

func verifyCreateClusterRole(cl client.Client) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: "test-name",
	}
	err := cl.Get(context.TODO(), nn, crole)
	Expect(err).NotTo(HaveOccurred())
	expectedRules := []rbacv1.PolicyRule{
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
				"watch",
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
				"test-name",
			},
			Verbs: []string{
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
	}
	Expect(crole.Rules).To(Equal(expectedRules))
}

func verifyCreateClusterRoleBinding(cl client.Client) {
	crb := &rbacv1.ClusterRoleBinding{}
	nn := types.NamespacedName{
		Name: "test-name",
	}
	err := cl.Get(context.TODO(), nn, crb)
	Expect(err).NotTo(HaveOccurred())
	Expect(crb.Subjects[0].Name).To(Equal("test-name-admin"))
	Expect(crb.Subjects[0].Namespace).To(Equal("test-namespace"))
}

func verifyCreateSCC(cl client.Client) {
	scc := &secv1.SecurityContextConstraints{}
	nn := types.NamespacedName{
		Name: "test-name",
	}
	err := cl.Get(context.TODO(), nn, scc)
	Expect(err).NotTo(HaveOccurred())
	expected := &secv1.SecurityContextConstraints{
		Groups: []string{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		// Meta data is dynamic, copy it so we can compare.
		ObjectMeta:               *scc.ObjectMeta.DeepCopy(),
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
			"system:serviceaccount:test-namespace:test-name-admin",
		},
		Volumes: []secv1.FSType{
			secv1.FSTypeHostPath,
			secv1.FSTypeSecret,
		},
	}
	Expect(scc).To(Equal(expected))
}
