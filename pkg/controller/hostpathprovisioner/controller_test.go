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
	"k8s.io/utils/ptr"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	secv1 "github.com/openshift/api/security/v1"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules/alerts"
)

const (
	versionString = "1.0.1"
	legacyVolume  = "pv-volume"
	socketDir     = "socket-dir"
	testNamespace = "test-namespace"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.BeforeEach(func() {
		watchNamespaceFunc = func() (string, error) {
			return testNamespace, nil
		}
		version.VersionStringFunc = func() (string, error) {
			return versionString, nil
		}
	})

	ginkgo.It("Should create new everything if nothing exist", func() {
		createDeployedCr(createLegacyCr())
	})

	ginkgo.DescribeTable("Should respect snapshot feature gate", func(cr *hppv1.HostPathProvisioner, scName string) {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		args := getDaemonSetArgs(logf.Log.WithName("hostpath-provisioner-operator-controller-test"), testNamespace, false)
		cr, r, cl := createDeployedCr(cr)
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      args.name,
				Namespace: testNamespace,
			},
		}
		csiVolume := fmt.Sprintf("%s-data-dir", scName)
		err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Ensure the csi side cars are there.
		sidecarImages := make([]string, 0)
		for _, container := range ds.Spec.Template.Spec.Containers {
			sidecarImages = append(sidecarImages, container.Image)
		}
		gomega.Expect(sidecarImages).To(gomega.ContainElements(CsiProvisionerImageDefault, CsiNodeDriverRegistrationImageDefault, LivenessProbeImageDefault, CsiSigStorageProvisionerImageDefault))
		// Ensure the snapshot sidecar is not there.
		gomega.Expect(sidecarImages).ToNot(gomega.ContainElement(SnapshotterImageDefault))
		found := false
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == csiVolume {
				found = true
			}
		}
		gomega.Expect(found).To(gomega.BeTrue(), fmt.Sprintf("%v", ds.Spec.Template.Spec.Volumes))

		cr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, cr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Update the CR to enable the snapshotting feature gate.
		cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
		err = cl.Update(context.TODO(), cr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		cr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, cr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cr.Spec.FeatureGates).To(gomega.ContainElement(snapshotFeatureGate))

		// Run the reconcile loop
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
		// Check the daemonSet value, make sure it added the snapshotter sidecar.
		ds = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      args.name,
				Namespace: testNamespace,
			},
		}
		err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Ensure the csi side cars are there.
		sidecarImages = make([]string, 0)
		for _, container := range ds.Spec.Template.Spec.Containers {
			sidecarImages = append(sidecarImages, container.Image)
		}
		gomega.Expect(sidecarImages).To(gomega.ContainElements(CsiProvisionerImageDefault, CsiNodeDriverRegistrationImageDefault, LivenessProbeImageDefault, CsiSigStorageProvisionerImageDefault, SnapshotterImageDefault))
		found = false
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == csiVolume {
				found = true
			}
		}
		gomega.Expect(found).To(gomega.BeTrue())
	},
		ginkgo.Entry("legacyCr", createLegacyCr(), "csi"),
		ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr(), "legacy"),
		ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr(), "local"),
	)

	ginkgo.It("Should requeue if watch namespaces returns error", func() {
		watchNamespaceFunc = func() (string, error) {
			return "", fmt.Errorf("Something is not right, no watch namespace")
		}
		cr := createLegacyCr()
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		promv1.AddToScheme(s)
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
				Namespace: testNamespace,
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
	})

	ginkgo.It("Should requeue if cr cannot be located", func() {
		cr := createLegacyCr()
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		promv1.AddToScheme(s)
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
				Namespace: testNamespace,
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
	})

	ginkgo.It("Should fail if trying to downgrade", func() {
		_, r, _ := createDeployedCr(createLegacyCr())
		version.VersionStringFunc = func() (string, error) {
			return "1.0.0", nil
		}
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
		gomega.Expect(strings.Contains(err.Error(), "downgraded")).To(gomega.BeTrue())
	})

	ginkgo.It("Should update CR status when upgrading", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		_, r, cl := createDeployedCr(createLegacyCr())
		version.VersionStringFunc = func() (string, error) {
			return "1.0.2", nil
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())

		updatedCr := &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal("1.0.2"))
		gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal("1.0.2"))
		gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal("1.0.2"))
		// Didn't make daemonset unavailable, so should be fully healthy
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeTrue())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeFalse())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeFalse())

		// Upgrade again, but make daemon set unavailable
		version.VersionStringFunc = func() (string, error) {
			return "1.0.3", nil
		}
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNN, ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Now make the csi daemonSet unavailable, and reconcile again.
		dsCsi := &appsv1.DaemonSet{}
		dsNNCsi := types.NamespacedName{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		dsCsi.Status.NumberReady = 1
		dsCsi.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), dsCsi)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		res, err = r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())

		updatedCr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal("1.0.3"))
		gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal("1.0.2"))
		gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal("1.0.3"))
		// Deployed, but NumberReady < DesiredNumberScheduled, so should be Available:False,Progressing:False,Degraded:True
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeFalse())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeFalse())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeTrue())

		ds = &appsv1.DaemonSet{}
		err = cl.Get(context.TODO(), dsNN, ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		ds.Status.NumberReady = 2
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		err = cl.Get(context.TODO(), dsNN, ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(ds.Status.NumberReady).To(gomega.Equal(int32(2)))
		gomega.Expect(ds.Status.DesiredNumberScheduled).To(gomega.Equal(int32(2)))
		dsCsi = &appsv1.DaemonSet{}
		dsNNCsi = types.NamespacedName{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		dsCsi.Status.NumberReady = 2
		dsCsi.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), dsCsi)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(dsCsi.Status.NumberReady).To(gomega.Equal(int32(2)))
		gomega.Expect(dsCsi.Status.DesiredNumberScheduled).To(gomega.Equal(int32(2)))

		res, err = r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())

		updatedCr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal("1.0.3"))
		gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal("1.0.3"))
		gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal("1.0.3"))
		// Didn't make daemonset unavailable, so should be fully healthy
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeTrue())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeFalse())
		// It should NOT be degraded
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeFalse())
	})

	ginkgo.It("Should delete CR name dependent resource when upgrading", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		_, r, _ := createDeployedCr(createLegacyCr())
		// Mimic a service account from a previous version whose name depends on the CR's
		err := r.client.Create(context.TODO(), createServiceAccountWithNameThatDependsOnCr())
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		saList := &corev1.ServiceAccountList{}
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: testNamespace})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(len(saList.Items)).To(gomega.Equal(3))
		gomega.Expect(saList.Items[1].Name).To(gomega.Equal(ProvisionerServiceAccountNameCsi))

		version.VersionStringFunc = func() (string, error) {
			return "1.0.2", nil
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())

		updatedCr := &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal("1.0.2"))
		gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal("1.0.2"))
		gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal("1.0.2"))

		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeTrue())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeFalse())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeFalse())

		saList = &corev1.ServiceAccountList{}
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: testNamespace})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(len(saList.Items)).To(gomega.Equal(2))
		gomega.Expect(saList.Items[0].Name).To(gomega.Equal(ProvisionerServiceAccountName))
	})

	ginkgo.It("Should err when more than one CR", func() {
		secondCr := &hppv1.HostPathProvisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name-second",
				Namespace: testNamespace,
			},
			Spec: hppv1.HostPathProvisionerSpec{
				ImagePullPolicy: corev1.PullAlways,
				PathConfig: &hppv1.PathConfig{
					Path:            "/tmp/test",
					UseNamingPrefix: false,
				},
			},
		}
		cr := createLegacyCr()
		objs := []runtime.Object{cr, secondCr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		promv1.AddToScheme(s)
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
				Namespace: testNamespace,
			},
		}
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
		gomega.Expect(err.Error()).To(gomega.Equal("there should be a single hostpath provisioner, 2 items found"))
	})

	ginkgo.It("Should not requeue when CR is deleted", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		cr, r, cl := createDeployedCr(createLegacyCr())
		err := cl.Delete(context.TODO(), cr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		res, err := r.Reconcile(context.TODO(), req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(res.Requeue).To(gomega.BeFalse())
	})

	ginkgo.It("Should update CR with FailedHealing", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		_, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		}
		err := cl.Get(context.TODO(), dsNN, ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		err = cl.Delete(context.TODO(), ds, &client.DeleteOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		newCl := erroringFakeCtrlRuntimeClient{
			Client: cl,
			errMsg: "create failed",
		}
		r.client = newCl
		_, err = r.Reconcile(context.TODO(), req)
		gomega.Expect(err).To(gomega.HaveOccurred())
		updatedCr := &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeFalse())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeTrue())
		gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeTrue())
		gomega.Expect(conditions.FindStatusCondition(updatedCr.Status.Conditions, conditions.ConditionDegraded).Message).To(gomega.Equal("Unable to successfully reconcile: create failed"))
	})
})

func createLegacyCr() *hppv1.HostPathProvisioner {
	return &hppv1.HostPathProvisioner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: testNamespace,
		},
		Spec: hppv1.HostPathProvisionerSpec{
			ImagePullPolicy: corev1.PullAlways,
			PathConfig: &hppv1.PathConfig{
				Path:            "/tmp/test",
				UseNamingPrefix: false,
			},
		},
	}
}

func createLegacyStoragePoolCr() *hppv1.HostPathProvisioner {
	return &hppv1.HostPathProvisioner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: testNamespace,
		},
		Spec: hppv1.HostPathProvisionerSpec{
			ImagePullPolicy: corev1.PullAlways,
			StoragePools: []hppv1.StoragePool{
				{
					Name: "legacy",
					Path: "/tmp/test",
				},
			},
		},
	}
}

func createStoragePoolWithTemplateLongNameCr() *hppv1.HostPathProvisioner {
	volumeMode := corev1.PersistentVolumeFilesystem
	name := "l123456789012345678901234567890123456789012345678901234567890123"
	gomega.Expect(len(name)).To(gomega.BeNumerically(">=", maxMountNameLength))
	return createStoragePoolWithTemplateVolumeModeCr(name, &volumeMode)
}

func createStoragePoolWithTemplateCr() *hppv1.HostPathProvisioner {
	volumeMode := corev1.PersistentVolumeFilesystem
	return createStoragePoolWithTemplateVolumeModeCr("local", &volumeMode)
}

func createStoragePoolWithTemplateBlockCr() *hppv1.HostPathProvisioner {
	volumeMode := corev1.PersistentVolumeBlock
	return createStoragePoolWithTemplateVolumeModeCr("local", &volumeMode)
}

func createStoragePoolWithTemplateVolumeModeCr(name string, volumeMode *corev1.PersistentVolumeMode) *hppv1.HostPathProvisioner {
	scName := "test"
	return &hppv1.HostPathProvisioner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: testNamespace,
		},
		Spec: hppv1.HostPathProvisionerSpec{
			ImagePullPolicy: corev1.PullAlways,
			StoragePools: []hppv1.StoragePool{
				{
					Name: name,
					Path: "/tmp/test",
					PVCTemplate: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &scName,
						VolumeMode:       volumeMode,
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}
}

func createStoragePoolWithTemplateVolumeModeAndBasicCr(name string, volumeMode *corev1.PersistentVolumeMode) *hppv1.HostPathProvisioner {
	cr := createStoragePoolWithTemplateVolumeModeCr(name, volumeMode)
	pvcTemplate := cr.Spec.StoragePools[0]
	cr.Spec.StoragePools = make([]hppv1.StoragePool, 0)
	cr.Spec.StoragePools = append(cr.Spec.StoragePools, hppv1.StoragePool{
		Name: "basic",
		Path: "/tmp/basic",
	})
	cr.Spec.StoragePools = append(cr.Spec.StoragePools, pvcTemplate)
	return cr
}

// After this has run, the returned cr state should be available, not progressing and not degraded.
func createDeployedCr(cr *hppv1.HostPathProvisioner) (*hppv1.HostPathProvisioner, *ReconcileHostPathProvisioner, client.Client) {
	objs := []runtime.Object{cr}
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
	s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
	promv1.AddToScheme(s)
	secv1.Install(s)
	ocpconfigv1.Install(s)

	// Create a fake client to mock API calls.
	cl := erroringFakeCtrlRuntimeClient{
		Client: fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build(),
		errMsg: "",
	}

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
			Namespace: testNamespace,
		},
	}
	res, err := r.Reconcile(context.TODO(), req)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(res.Requeue).To(gomega.BeFalse())
	updatedCr := &hppv1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal(versionString))
	gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal(versionString))
	gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal(""))
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeFalse())
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeTrue())
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeFalse())
	// Verify all the different objects are created.
	if r.isLegacy(cr) {
		verifyCreateDaemonSet(r.client)
		verifyCreateServiceAccount(r.client, ProvisionerServiceAccountName)
		verifyCreateClusterRole(r.client)
		verifyCreateClusterRoleBinding(r.client)
	}
	verifyCreateDaemonSetCsi(r.client)
	verifyCreateServiceAccount(r.client, ProvisionerServiceAccountNameCsi)
	verifyCreateCSIClusterRole(r.client, false)
	verifyCreateCSIClusterRoleBinding(r.client)
	verifyCreateCSIRole(r.client)
	verifyCreateCSIRoleBinding(r.client)
	verifyCreateCSIDriver(r.client)
	verifyCreateSCC(r.client)
	verifyCreatePrometheusResources(r.client)

	if r.isLegacy(cr) {
		// Now make the daemonSet available, and reconcile again.
		ds := &appsv1.DaemonSet{}
		dsNN := types.NamespacedName{
			Name:      MultiPurposeHostPathProvisionerName,
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNN, ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		ds.Status.NumberReady = 2
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), ds)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
	// Now make the csi daemonSet available, and reconcile again.
	dsCsi := &appsv1.DaemonSet{}
	dsNNCsi := types.NamespacedName{
		Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	dsCsi.Status.NumberReady = 2
	dsCsi.Status.DesiredNumberScheduled = 2
	err = cl.Status().Update(context.TODO(), dsCsi)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// daemonSet is ready, now reconcile again. We should have condition changes and observed version should be set.
	res, err = r.Reconcile(context.TODO(), req)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(res.Requeue).To(gomega.BeFalse())
	updatedCr = &hppv1.HostPathProvisioner{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(updatedCr.Status.OperatorVersion).To(gomega.Equal(versionString))
	gomega.Expect(updatedCr.Status.TargetVersion).To(gomega.Equal(versionString))
	gomega.Expect(updatedCr.Status.ObservedVersion).To(gomega.Equal(versionString))
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(gomega.BeTrue())
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(gomega.BeFalse())
	gomega.Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(gomega.BeFalse())
	return cr, r, cl
}

// Verify all the proper values are set when creating the daemonset
func verifyCreateDaemonSet(cl client.Client) {
	ds := &appsv1.DaemonSet{}
	nn := types.NamespacedName{
		Name:      MultiPurposeHostPathProvisionerName,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, ds)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	// Check Service Account
	gomega.Expect(ds.Spec.Template.Spec.ServiceAccountName).To(gomega.Equal(ProvisionerServiceAccountName))
	// Check k8s recommended labels
	gomega.Expect(ds.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
	gomega.Expect(ds.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal(ProvisionerImageDefault))
	// No junk in .spec.selector, should only be a minimal set that is needed to know which pods are under our governance
	gomega.Expect(ds.Spec.Selector.MatchLabels).To(gomega.Equal(
		map[string]string{
			"k8s-app": MultiPurposeHostPathProvisionerName,
		},
	))
	// Check use naming prefix
	gomega.Expect(ds.Spec.Template.Spec.Containers[0].Env[0].Value).To(gomega.Equal("false"))
	// Check directory
	gomega.Expect(ds.Spec.Template.Spec.Containers[0].Env[2].Value).To(gomega.Equal("/tmp/test"))
}

// Verify all the proper values are set when creating the daemonset
func verifyCreateDaemonSetCsi(cl client.Client) {
	ds := &appsv1.DaemonSet{}
	nn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, ds)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	// Check Service Account
	gomega.Expect(ds.Spec.Template.Spec.ServiceAccountName).To(gomega.Equal(ProvisionerServiceAccountNameCsi))
	// Check k8s recommended labels
	gomega.Expect(ds.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
	gomega.Expect(ds.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal(CsiProvisionerImageDefault))
	// No junk in .spec.selector, should only be a minimal set that is needed to know which pods are under our governance
	gomega.Expect(ds.Spec.Selector.MatchLabels).To(gomega.Equal(
		map[string]string{
			"k8s-app": MultiPurposeHostPathProvisionerName,
		},
	))
}

func verifyCreateServiceAccount(cl client.Client, name string) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(sa), sa)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(sa.ObjectMeta.Name).To(gomega.Equal(name))
	gomega.Expect(sa.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateCSIClusterRole(cl client.Client, enableSnapshot bool) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: ProvisionerServiceAccountNameCsi,
	}
	err := cl.Get(context.TODO(), nn, crole)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
	if enableSnapshot {
		expectedRules = append(expectedRules, createSnapshotCsiClusterRoles()...)
	}

	gomega.Expect(crole.Rules).To(gomega.Equal(expectedRules))
	gomega.Expect(crole.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateClusterRole(cl client.Client) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: MultiPurposeHostPathProvisionerName,
	}
	err := cl.Get(context.TODO(), nn, crole)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
	gomega.Expect(crole.Rules).To(gomega.Equal(expectedRules))
	gomega.Expect(crole.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateCSIRole(cl client.Client) {
	role := &rbacv1.Role{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountNameCsi,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, role)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
	gomega.Expect(role.Rules).To(gomega.Equal(expectedRules))
	gomega.Expect(role.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateClusterRoleBinding(cl client.Client) {
	crb := &rbacv1.ClusterRoleBinding{}
	nn := types.NamespacedName{
		Name: MultiPurposeHostPathProvisionerName,
	}
	err := cl.Get(context.TODO(), nn, crb)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(crb.Subjects[0].Name).To(gomega.Equal(ProvisionerServiceAccountName))
	gomega.Expect(crb.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateCSIClusterRoleBinding(cl client.Client) {
	crb := &rbacv1.ClusterRoleBinding{}
	nn := types.NamespacedName{
		Name: ProvisionerServiceAccountNameCsi,
	}
	err := cl.Get(context.TODO(), nn, crb)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(crb.Subjects[0].Name).To(gomega.Equal(ProvisionerServiceAccountNameCsi))
	gomega.Expect(crb.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateCSIRoleBinding(cl client.Client) {
	rb := &rbacv1.RoleBinding{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountNameCsi,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, rb)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(rb.Subjects[0].Name).To(gomega.Equal(ProvisionerServiceAccountNameCsi))
	gomega.Expect(rb.Subjects[0].Namespace).To(gomega.Equal(testNamespace))
	gomega.Expect(rb.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateSCC(cl client.Client) {
	scc := &secv1.SecurityContextConstraints{}
	nn := types.NamespacedName{
		Name: fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
	}
	err := cl.Get(context.TODO(), nn, scc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	expected := &secv1.SecurityContextConstraints{
		Groups: []string{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		// Meta data is dynamic, copy it so we can compare.
		ObjectMeta:               *scc.ObjectMeta.DeepCopy(),
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
			fmt.Sprintf("system:serviceaccount:test-namespace:%s-csi", ProvisionerServiceAccountName),
		},
		Volumes: []secv1.FSType{
			secv1.FSTypeAll,
		},
	}
	gomega.Expect(scc).To(gomega.Equal(expected))
	gomega.Expect(scc.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreatePrometheusResources(cl client.Client) {
	// PrometheusRule
	rule := &promv1.PrometheusRule{}
	nn := types.NamespacedName{
		Name:      ruleName,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, rule)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for _, r := range rule.Spec.Groups[0].Rules {
		gomega.Expect(r.Annotations).To(gomega.BeNil())
	}

	runbookURLTemplate := alerts.GetRunbookURLTemplate()

	hppDownAlert := promv1.Rule{
		Alert: "HPPOperatorDown",
		Expr:  intstr.FromString("kubevirt_hpp_operator_up == 0"),
		For:   ptr.To[promv1.Duration]("5m"),
		Annotations: map[string]string{
			"summary":     "Hostpath Provisioner operator is down.",
			"runbook_url": fmt.Sprintf(runbookURLTemplate, "HPPOperatorDown"),
		},
		Labels: map[string]string{
			"severity":                      "warning",
			"operator_health_impact":        "critical",
			"kubernetes_operator_part_of":   "kubevirt",
			"kubernetes_operator_component": "hostpath-provisioner-operator",
		},
	}
	gomega.Expect(rule.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
	gomega.Expect(rule.Spec.Groups[1].Rules).To(gomega.ContainElement(hppDownAlert))

	// ServiceMonitor
	monitor := &promv1.ServiceMonitor{}
	nn = types.NamespacedName{
		Name:      monitorName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, monitor)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(monitor.Spec.NamespaceSelector.MatchNames).To(gomega.ContainElement(testNamespace))
	gomega.Expect(rule.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))

	// RBAC
	role := &rbacv1.Role{}
	roleBinding := &rbacv1.RoleBinding{}
	nn = types.NamespacedName{
		Name:      rbacName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, role)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(role.Rules[0].Resources).To(gomega.ContainElement("endpoints"))
	gomega.Expect(rule.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
	err = cl.Get(context.TODO(), nn, roleBinding)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(roleBinding.Subjects[0].Name).To(gomega.Equal("prometheus-k8s"))
	gomega.Expect(roleBinding.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))

	// Service
	service := &corev1.Service{}
	nn = types.NamespacedName{
		Name:      PrometheusServiceName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, service)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	port := corev1.ServicePort{
		Name: "metrics",
		Port: 8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "metrics",
		},
		Protocol: corev1.ProtocolTCP,
	}
	gomega.Expect(service.Spec.Ports).To(gomega.ContainElement(port))
	gomega.Expect(service.Spec.Selector).To(gomega.Equal(map[string]string{PrometheusLabelKey: PrometheusLabelValue}))
	gomega.Expect(service.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
}

func verifyCreateCSIDriver(cl client.Client) {
	cd := &storagev1.CSIDriver{}
	nn := types.NamespacedName{
		Name: driverName,
	}
	err := cl.Get(context.TODO(), nn, cd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(cd.Spec.AttachRequired).NotTo(gomega.BeNil())
	gomega.Expect(*cd.Spec.AttachRequired).To(gomega.BeFalse())
	gomega.Expect(cd.Spec.PodInfoOnMount).NotTo(gomega.BeNil())
	gomega.Expect(*cd.Spec.PodInfoOnMount).To(gomega.BeTrue())
	gomega.Expect(len(cd.Spec.VolumeLifecycleModes)).To(gomega.Equal(2))
	gomega.Expect(cd.Spec.VolumeLifecycleModes).To(gomega.ContainElements(storagev1.VolumeLifecyclePersistent, storagev1.VolumeLifecycleEphemeral))
}

func createServiceAccountWithNameThatDependsOnCr() *corev1.ServiceAccount {
	labels := map[string]string{
		"k8s-app": MultiPurposeHostPathProvisionerName,
	}
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name-admin",
			Namespace: testNamespace,
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

type erroringFakeCtrlRuntimeClient struct {
	client.Client
	errMsg string
}

func (p erroringFakeCtrlRuntimeClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if len(p.errMsg) > 0 {
		return fmt.Errorf(p.errMsg)
	}
	return p.Client.Create(ctx, obj, opts...)
}
