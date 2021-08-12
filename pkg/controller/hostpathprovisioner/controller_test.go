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
	"k8s.io/client-go/tools/record"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	conditions "github.com/openshift/custom-resource-status/conditions/v1"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	versionString = "1.0.1"
)

var _ = Describe("Controller reconcile loop", func() {
	var (
		cr *hppv1.HostPathProvisioner
		cl client.Client
		r  *ReconcileHostPathProvisioner
	)
	BeforeEach(func() {
		watchNamespaceFunc = func() (string, error) {
			return "test-namespace", nil
		}
		version.VersionStringFunc = func() (string, error) {
			return versionString, nil
		}
		cr = &hppv1.HostPathProvisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
			Spec: hppv1.HostPathProvisionerSpec{
				ImagePullPolicy: corev1.PullAlways,
				PathConfig: hppv1.PathConfig{
					Path:            "/tmp/test",
					UseNamingPrefix: false,
				},
			},
		}
	})

	table.DescribeTable("Should create new everything if nothing exist", func(DisableCsi bool) {
		cr.Spec.DisableCsi = DisableCsi
		createDeployedCr(cr)
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	table.DescribeTable("Should fix a changed daemonSet", func(DisableCsi bool) {
		cr.Spec.DisableCsi = DisableCsi
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: "test-namespace",
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the daemonSet to something not desired.
		ds := &appsv1.DaemonSet{}
		err := cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Spec.Template.Spec.Volumes[0].Name = "invalid"
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())
		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal("invalid"))

		// Run the reconcile loop
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Check the daemonSet value, make sure it changed back.
		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		if DisableCsi {
			Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal("pv-volume"))
		} else {
			Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal("csi-data-dir"))
		}
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	table.DescribeTable("Should fix a changed service account", func(DisableCsi bool) {
		cr.Spec.DisableCsi = DisableCsi
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		saNN := types.NamespacedName{
			Name:      ProvisionerServiceAccountName,
			Namespace: "test-namespace",
		}
		cr, r, cl = createDeployedCr(cr)
		// Now modify the service account to something not desired.
		sa := &corev1.ServiceAccount{}
		err := cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.ObjectMeta.Labels["k8s-app"]).To(Equal(MultiPurposeHostPathProvisionerName))
		sa.ObjectMeta.Labels["k8s-app"] = "invalid"
		err = cl.Update(context.TODO(), sa)
		Expect(err).NotTo(HaveOccurred())
		sa = &corev1.ServiceAccount{}
		err = cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.ObjectMeta.Labels["k8s-app"]).To(Equal("invalid"))
		// Run the reconcile loop
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify the label has been changed back.
		sa = &corev1.ServiceAccount{}
		err = cl.Get(context.TODO(), saNN, sa)
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.ObjectMeta.Labels["k8s-app"]).To(Equal(MultiPurposeHostPathProvisionerName))
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	table.DescribeTable("Should fix a changed ClusterRole", func(DisableCsi bool) {
		cr.Spec.DisableCsi = DisableCsi
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		name := MultiPurposeHostPathProvisionerName
		if !DisableCsi {
			name = ProvisionerServiceAccountName
		}
		croleNN := types.NamespacedName{
			Name: name,
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
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify its gone now
		err = cl.Get(context.TODO(), croleNN, crole)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crole.Rules[1].Verbs)).To(Equal(4))
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	table.DescribeTable("Should fix a changed ClusterRoleBinding", func(DisableCsi bool) {
		cr.Spec.DisableCsi = DisableCsi
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		name := ProvisionerServiceAccountName
		if cr.Spec.DisableCsi {
			name = MultiPurposeHostPathProvisionerName
		}
		crbNN := types.NamespacedName{
			Name: name,
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
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify the name is correct again.
		crb = &rbacv1.ClusterRoleBinding{}
		err = cl.Get(context.TODO(), crbNN, crb)
		Expect(err).NotTo(HaveOccurred())
		Expect(crb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountName))
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	table.DescribeTable("Should fix a changed SecurityContextConstraints", func(disableCsi bool) {
		cr.Spec.DisableCsi = disableCsi
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		sccNN := types.NamespacedName{
			Name: MultiPurposeHostPathProvisionerName,
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
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Verify allowPrivileged is false
		scc = &secv1.SecurityContextConstraints{}
		err = cl.Get(context.TODO(), sccNN, scc)
		Expect(err).NotTo(HaveOccurred())
		Expect(scc.AllowPrivilegedContainer).To(Equal(!disableCsi))
		if !disableCsi {
			Expect(scc.Volumes).To(BeEmpty())
		} else {
			Expect(scc.Volumes).To(ContainElements(secv1.FSTypeHostPath, secv1.FSTypeSecret, secv1.FSProjected))
		}
	},
		table.Entry("Disable CSI", true),
		table.Entry("Enable CSI", false),
	)

	It("Should requeue if watch namespaces returns error", func() {
		watchNamespaceFunc = func() (string, error) {
			return "", fmt.Errorf("Something is not right, no watch namespace")
		}
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		secv1.Install(s)

		// Create a fake client to mock API calls.
		cl := fake.NewFakeClient(objs...)

		// Create a ReconcileMemcached object with the scheme and fake client.
		r := &ReconcileHostPathProvisioner{
			client:   cl,
			scheme:   s,
			recorder: record.NewFakeRecorder(250),
			Log:      logf.Log.WithName("hostpath-provisioner-operator-controller-test"),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should requeue if cr cannot be located", func() {
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		secv1.Install(s)

		// Create a fake client to mock API calls.
		cl := fake.NewFakeClient(objs...)

		// Create a ReconcileMemcached object with the scheme and fake client.
		r := &ReconcileHostPathProvisioner{
			client:   cl,
			scheme:   s,
			recorder: record.NewFakeRecorder(250),
			Log:      logf.Log.WithName("hostpath-provisioner-operator-controller-test"),
		}

		// Mock request to simulate Reconcile() being called on an event for a
		// watched resource .
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name2",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should fail if trying to downgrade", func() {
		cr, r, cl = createDeployedCr(cr)
		version.VersionStringFunc = func() (string, error) {
			return "1.0.0", nil
		}
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
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
		version.VersionStringFunc = func() (string, error) {
			return "1.0.2", nil
		}
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr := &hppv1.HostPathProvisioner{}
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
		version.VersionStringFunc = func() (string, error) {
			return "1.0.3", nil
		}
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: "test-namespace",
		}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())

		res, err = r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedCr.Status.OperatorVersion).To(Equal("1.0.3"))
		Expect(updatedCr.Status.ObservedVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.TargetVersion).To(Equal("1.0.3"))
		// Didn't make daemonset unavailable, so should be fully healthy
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
		// It should be degraded
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 2
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())

		res, err = r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr = &hppv1.HostPathProvisioner{}
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

	It("Should delete CR name dependent resource when upgrading", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		// Mimic a service account from a previous version whose name depends on the CR's
		err := r.client.Create(context.TODO(), createServiceAccountWithNameThatDependsOnCr())
		Expect(err).NotTo(HaveOccurred())
		saList := &corev1.ServiceAccountList{}
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: "test-namespace"})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(saList.Items)).To(Equal(2))
		Expect(saList.Items[1].Name).To(Equal("test-name-admin"))

		version.VersionStringFunc = func() (string, error) {
			return "1.0.2", nil
		}
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		updatedCr := &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedCr.Status.OperatorVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.ObservedVersion).To(Equal("1.0.2"))
		Expect(updatedCr.Status.TargetVersion).To(Equal("1.0.2"))

		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeFalse())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())

		saList = &corev1.ServiceAccountList{}
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: "test-namespace"})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(saList.Items)).To(Equal(1))
		Expect(saList.Items[0].Name).To(Equal(ProvisionerServiceAccountName))
	})

	It("Should err when more than one CR", func() {
		secondCr := &hppv1.HostPathProvisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name-second",
				Namespace: "test-namespace",
			},
			Spec: hppv1.HostPathProvisionerSpec{
				ImagePullPolicy: corev1.PullAlways,
				PathConfig: hppv1.PathConfig{
					Path:            "/tmp/test",
					UseNamingPrefix: false,
				},
			},
		}
		objs := []runtime.Object{cr, secondCr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		secv1.Install(s)

		// Create a fake client to mock API calls.
		cl := fake.NewFakeClient(objs...)

		// Create a ReconcileMemcached object with the scheme and fake client.
		r := &ReconcileHostPathProvisioner{
			client:   cl,
			scheme:   s,
			recorder: record.NewFakeRecorder(250),
			Log:      logf.Log.WithName("hostpath-provisioner-operator-controller-test"),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		Expect(err.Error()).To(Equal("there should be a single hostpath provisioner, 2 items found"))
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
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should create daemonset with node placement", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		cr, r, cl = createDeployedCr(cr)
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: "test-namespace",
		}
		err := cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())

		Expect(ds.Spec.Template.Spec.Affinity).To(BeNil())
		Expect(ds.Spec.Template.Spec.NodeSelector).To(BeEmpty())
		Expect(ds.Spec.Template.Spec.Tolerations).To(BeEmpty())

		affinityTestValue := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"somehostname"}},
							},
						},
					},
				},
			},
		}
		nodeSelectorTestValue := map[string]string{"kubernetes.io/arch": "ppc64le"}
		tolerationsTestValue := []corev1.Toleration{{Key: "test", Value: "123"}}

		cr = &hppv1.HostPathProvisioner{}
		err = cl.Get(context.TODO(), req.NamespacedName, cr)
		Expect(err).NotTo(HaveOccurred())
		cr.Spec.Workload = hppv1.NodePlacement{
			NodeSelector: nodeSelectorTestValue,
			Affinity:     affinityTestValue,
			Tolerations:  tolerationsTestValue,
		}
		err = cl.Update(context.TODO(), cr)
		Expect(err).NotTo(HaveOccurred())
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())

		Expect(ds.Spec.Template.Spec.Affinity).To(Equal(affinityTestValue))
		Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(nodeSelectorTestValue))
		Expect(ds.Spec.Template.Spec.Tolerations).To(Equal(tolerationsTestValue))
	})

	It("Should be able to remove node placement if CR doesn't have it anymore", func() {
		affinityTestValue := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"somehostname"}},
							},
						},
					},
				},
			},
		}
		nodeSelectorTestValue := map[string]string{"kubernetes.io/arch": "ppc64le"}
		tolerationsTestValue := []corev1.Toleration{{Key: "test", Value: "123"}}
		cr.Spec.Workload = hppv1.NodePlacement{
			NodeSelector: nodeSelectorTestValue,
			Affinity:     affinityTestValue,
			Tolerations:  tolerationsTestValue,
		}
		cr, r, cl = createDeployedCr(cr)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: "test-namespace",
		}
		err := cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())

		Expect(ds.Spec.Template.Spec.Affinity).To(Equal(affinityTestValue))
		Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(nodeSelectorTestValue))
		Expect(ds.Spec.Template.Spec.Tolerations).To(Equal(tolerationsTestValue))

		cr = &hppv1.HostPathProvisioner{}
		err = cl.Get(context.TODO(), req.NamespacedName, cr)
		Expect(err).NotTo(HaveOccurred())
		cr.Spec.Workload = hppv1.NodePlacement{}
		err = cl.Update(context.TODO(), cr)
		Expect(err).NotTo(HaveOccurred())
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())

		Expect(ds.Spec.Template.Spec.Affinity).To(BeNil())
		Expect(ds.Spec.Template.Spec.NodeSelector).To(BeEmpty())
		Expect(ds.Spec.Template.Spec.Tolerations).To(BeEmpty())
	})
})

// After this has run, the returned cr state should be available, not progressing and not degraded.
func createDeployedCr(cr *hppv1.HostPathProvisioner) (*hppv1.HostPathProvisioner, *ReconcileHostPathProvisioner, client.Client) {
	objs := []runtime.Object{cr}
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
	s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
	secv1.Install(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ReconcileHostPathProvisioner{
		client:   cl,
		scheme:   s,
		recorder: record.NewFakeRecorder(250),
		Log:      logf.Log.WithName("hostpath-provisioner-operator-controller-test"),
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
	}
	res, err := r.Reconcile(context.TODO(), req)
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Requeue).To(BeFalse())
	updatedCr := &hppv1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
	Expect(err).NotTo(HaveOccurred())
	Expect(updatedCr.Status.OperatorVersion).To(Equal(versionString))
	Expect(updatedCr.Status.TargetVersion).To(Equal(versionString))
	Expect(updatedCr.Status.ObservedVersion).To(Equal(""))
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeFalse())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
	Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeFalse())
	// Verify all the different objects are created.
	verifyCreateDaemonSet(r.client, cr.Spec.DisableCsi)
	verifyCreateServiceAccount(r.client)
	if cr.Spec.DisableCsi {
		verifyCreateClusterRole(r.client)
		verifyCreateClusterRoleBinding(r.client)
	} else {
		verifyCreateCSIClusterRole(r.client)
		verifyCreateCSIClusterRoleBinding(r.client)
		verifyCreateCSIRole(r.client)
		verifyCreateCSIRoleBinding(r.client)
		verifyCreateCSIDriver(r.client)
	}
	verifyCreateSCC(r.client, cr.Spec.DisableCsi)

	// Now make the daemonSet available, and reconcile again.
	ds := &appsv1.DaemonSet{}
	dsNN := types.NamespacedName{
		Name:      MultiPurposeHostPathProvisionerName,
		Namespace: "test-namespace",
	}
	err = cl.Get(context.TODO(), dsNN, ds)
	Expect(err).NotTo(HaveOccurred())
	ds.Status.NumberReady = 2
	ds.Status.DesiredNumberScheduled = 2
	err = cl.Update(context.TODO(), ds)
	Expect(err).NotTo(HaveOccurred())

	// daemonSet is ready, now reconcile again. We should have condition changes and observed version should be set.
	res, err = r.Reconcile(context.TODO(), req)
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Requeue).To(BeFalse())
	updatedCr = &hppv1.HostPathProvisioner{}
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
func verifyCreateDaemonSet(cl client.Client, DisableCsi bool) {
	ds := &appsv1.DaemonSet{}
	nn := types.NamespacedName{
		Name:      MultiPurposeHostPathProvisionerName,
		Namespace: "test-namespace",
	}
	err := cl.Get(context.TODO(), nn, ds)
	Expect(err).NotTo(HaveOccurred())
	// Check Service Account
	Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal(ProvisionerServiceAccountName))
	// Check k8s recommended labels
	Expect(ds.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
	if DisableCsi {
		Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(ProvisionerImageDefault))
		// Check use naming prefix
		Expect(ds.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("false"))
		// Check directory
		Expect(ds.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal("/tmp/test"))
	} else {
		Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(CsiProvisionerImageDefault))
	}
}

func verifyCreateServiceAccount(cl client.Client) {
	sa := &corev1.ServiceAccount{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountName,
		Namespace: "test-namespace",
	}
	err := cl.Get(context.TODO(), nn, sa)
	Expect(err).NotTo(HaveOccurred())
	Expect(sa.ObjectMeta.Name).To(Equal(ProvisionerServiceAccountName))
	Expect(sa.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIClusterRole(cl client.Client) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: ProvisionerServiceAccountName,
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
	}
	Expect(crole.Rules).To(Equal(expectedRules))
	Expect(crole.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))

	crole = &rbacv1.ClusterRole{}
	nn = types.NamespacedName{
		Name: healthCheckName,
	}
	err = cl.Get(context.TODO(), nn, crole)
	Expect(err).NotTo(HaveOccurred())
	expectedRules = []rbacv1.PolicyRule{
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
	}
	Expect(crole.Rules).To(Equal(expectedRules))
	Expect(crole.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateClusterRole(cl client.Client) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: MultiPurposeHostPathProvisionerName,
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
	}
	Expect(crole.Rules).To(Equal(expectedRules))
	Expect(crole.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIRole(cl client.Client) {
	role := &rbacv1.Role{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountName,
		Namespace: "test-namespace",
	}
	err := cl.Get(context.TODO(), nn, role)
	Expect(err).NotTo(HaveOccurred())
	expectedRules := []rbacv1.PolicyRule{
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
	}
	Expect(role.Rules).To(Equal(expectedRules))
	Expect(role.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))

	role = &rbacv1.Role{}
	nn = types.NamespacedName{
		Name:      healthCheckName,
		Namespace: "test-namespace",
	}
	err = cl.Get(context.TODO(), nn, role)
	Expect(err).NotTo(HaveOccurred())
	expectedRules = []rbacv1.PolicyRule{
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
	}
	Expect(role.Rules).To(Equal(expectedRules))
	Expect(role.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateClusterRoleBinding(cl client.Client) {
	crb := &rbacv1.ClusterRoleBinding{}
	nn := types.NamespacedName{
		Name: MultiPurposeHostPathProvisionerName,
	}
	err := cl.Get(context.TODO(), nn, crb)
	Expect(err).NotTo(HaveOccurred())
	Expect(crb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountName))
	Expect(crb.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIClusterRoleBinding(cl client.Client) {
	crb := &rbacv1.ClusterRoleBinding{}
	nn := types.NamespacedName{
		Name: ProvisionerServiceAccountName,
	}
	err := cl.Get(context.TODO(), nn, crb)
	Expect(err).NotTo(HaveOccurred())
	Expect(crb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountName))
	Expect(crb.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIRoleBinding(cl client.Client) {
	rb := &rbacv1.RoleBinding{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountName,
		Namespace: "test-namespace",
	}
	err := cl.Get(context.TODO(), nn, rb)
	Expect(err).NotTo(HaveOccurred())
	Expect(rb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountName))
	Expect(rb.Subjects[0].Namespace).To(Equal("test-namespace"))
	Expect(rb.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateSCC(cl client.Client, disableCsi bool) {
	scc := &secv1.SecurityContextConstraints{}
	nn := types.NamespacedName{
		Name: MultiPurposeHostPathProvisionerName,
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
		AllowPrivilegedContainer: !disableCsi,
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
			fmt.Sprintf("system:serviceaccount:test-namespace:%s", ProvisionerServiceAccountName),
		},
	}
	if disableCsi {
		expected.Volumes = []secv1.FSType{
			secv1.FSTypeHostPath,
			secv1.FSTypeSecret,
			secv1.FSProjected,
		}
	}
	Expect(scc).To(Equal(expected))
	Expect(scc.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIDriver(cl client.Client) {
	cd := &storagev1.CSIDriver{}
	nn := types.NamespacedName{
		Name: driverName,
	}
	err := cl.Get(context.TODO(), nn, cd)
	Expect(err).NotTo(HaveOccurred())
	Expect(cd.Spec.AttachRequired).NotTo(BeNil())
	Expect(*cd.Spec.AttachRequired).To(BeFalse())
	Expect(cd.Spec.PodInfoOnMount).NotTo(BeNil())
	Expect(*cd.Spec.PodInfoOnMount).To(BeTrue())
	Expect(len(cd.Spec.VolumeLifecycleModes)).To(Equal(1))
	Expect(cd.Spec.VolumeLifecycleModes[0]).To(Equal(storagev1.VolumeLifecyclePersistent))
}

func createServiceAccountWithNameThatDependsOnCr() *corev1.ServiceAccount {
	labels := map[string]string{
		"k8s-app": MultiPurposeHostPathProvisionerName,
	}
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-admin",
			Namespace: "test-namespace",
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "hostpathprovisioner.kubevirt.io/test",
					Kind:       "HostPathProvisioner",
					Name:       "test-name",
					UID:        "1234",
				},
			},
		},
	}
}
