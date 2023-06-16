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

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	secv1 "github.com/openshift/api/security/v1"
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
	legacyVolume  = "pv-volume"
	socketDir     = "socket-dir"
	testNamespace = "test-namespace"
)

var _ = Describe("Controller reconcile loop", func() {
	BeforeEach(func() {
		watchNamespaceFunc = func() (string, error) {
			return testNamespace, nil
		}
		version.VersionStringFunc = func() (string, error) {
			return versionString, nil
		}
	})

	It("Should create new everything if nothing exist", func() {
		createDeployedCr(createLegacyCr())
	})

	table.DescribeTable("Should respect snapshot feature gate", func(cr *hppv1.HostPathProvisioner, scName string) {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		args := getDaemonSetArgs(logf.Log.WithName("hostpath-provisioner-operator-controller-test"), testNamespace, false)
		_, r, cl := createDeployedCr(cr)
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      args.name,
				Namespace: testNamespace,
			},
		}
		csiVolume := fmt.Sprintf("%s-data-dir", scName)
		err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
		Expect(err).NotTo(HaveOccurred())
		// Ensure the csi side cars are there.
		sidecarImages := make([]string, 0)
		for _, container := range ds.Spec.Template.Spec.Containers {
			sidecarImages = append(sidecarImages, container.Image)
		}
		Expect(sidecarImages).To(ContainElements(CsiProvisionerImageDefault, CsiNodeDriverRegistrationImageDefault, LivenessProbeImageDefault, CsiSigStorageProvisionerImageDefault))
		// Ensure the snapshot sidecar is not there.
		Expect(sidecarImages).ToNot(ContainElement(SnapshotterImageDefault))
		found := false
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == csiVolume {
				found = true
			}
		}
		Expect(found).To(BeTrue(), fmt.Sprintf("%v", ds.Spec.Template.Spec.Volumes))

		cr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, cr)
		Expect(err).NotTo(HaveOccurred())

		// Update the CR to enable the snapshotting feature gate.
		cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
		err = cl.Update(context.TODO(), cr)
		Expect(err).NotTo(HaveOccurred())

		cr = &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(cr.Spec.FeatureGates).To(ContainElement(snapshotFeatureGate))

		// Run the reconcile loop
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		// Check the daemonSet value, make sure it added the snapshotter sidecar.
		ds = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      args.name,
				Namespace: testNamespace,
			},
		}
		err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
		Expect(err).NotTo(HaveOccurred())
		// Ensure the csi side cars are there.
		sidecarImages = make([]string, 0)
		for _, container := range ds.Spec.Template.Spec.Containers {
			sidecarImages = append(sidecarImages, container.Image)
		}
		Expect(sidecarImages).To(ContainElements(CsiProvisionerImageDefault, CsiNodeDriverRegistrationImageDefault, LivenessProbeImageDefault, CsiSigStorageProvisionerImageDefault, SnapshotterImageDefault))
		found = false
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == csiVolume {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	},
		table.Entry("legacyCr", createLegacyCr(), "csi"),
		table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr(), "legacy"),
		table.Entry("storagePoolCr", createStoragePoolWithTemplateCr(), "local"),
	)

	It("Should requeue if watch namespaces returns error", func() {
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
		cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

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
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should requeue if cr cannot be located", func() {
		cr := createLegacyCr()
		objs := []runtime.Object{cr}
		// Register operator types with the runtime scheme.
		s := scheme.Scheme
		s.AddKnownTypes(hppv1.SchemeGroupVersion, cr)
		s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
		promv1.AddToScheme(s)
		secv1.Install(s)

		// Create a fake client to mock API calls.
		cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

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
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should fail if trying to downgrade", func() {
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
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		Expect(strings.Contains(err.Error(), "downgraded")).To(BeTrue())
	})

	It("Should update CR status when upgrading", func() {
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
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNN, ds)
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 1
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())
		// Now make the csi daemonSet unavailable, and reconcile again.
		dsCsi := &appsv1.DaemonSet{}
		dsNNCsi := types.NamespacedName{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
		Expect(err).NotTo(HaveOccurred())
		dsCsi.Status.NumberReady = 1
		dsCsi.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), dsCsi)
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
		dsCsi = &appsv1.DaemonSet{}
		dsNNCsi = types.NamespacedName{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		}
		err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
		Expect(err).NotTo(HaveOccurred())
		dsCsi.Status.NumberReady = 2
		dsCsi.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), dsCsi)
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
				Namespace: testNamespace,
			},
		}
		_, r, _ := createDeployedCr(createLegacyCr())
		// Mimic a service account from a previous version whose name depends on the CR's
		err := r.client.Create(context.TODO(), createServiceAccountWithNameThatDependsOnCr())
		Expect(err).NotTo(HaveOccurred())
		saList := &corev1.ServiceAccountList{}
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: testNamespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(saList.Items)).To(Equal(3))
		Expect(saList.Items[1].Name).To(Equal(ProvisionerServiceAccountNameCsi))

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
		err = r.client.List(context.TODO(), saList, &client.ListOptions{Namespace: testNamespace})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(saList.Items)).To(Equal(2))
		Expect(saList.Items[0].Name).To(Equal(ProvisionerServiceAccountName))
	})

	It("Should err when more than one CR", func() {
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
		cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

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
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
		Expect(err.Error()).To(Equal("there should be a single hostpath provisioner, 2 items found"))
	})

	It("Should not requeue when CR is deleted", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: testNamespace,
			},
		}
		cr, r, cl := createDeployedCr(createLegacyCr())
		err := cl.Delete(context.TODO(), cr)
		Expect(err).NotTo(HaveOccurred())
		res, err := r.Reconcile(context.TODO(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("Should update CR with FailedHealing", func() {
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
		Expect(err).NotTo(HaveOccurred())
		err = cl.Delete(context.TODO(), ds, &client.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		newCl := erroringFakeCtrlRuntimeClient{
			Client: cl,
			errMsg: "create failed",
		}
		r.client = newCl
		_, err = r.Reconcile(context.TODO(), req)
		Expect(err).To(HaveOccurred())
		updatedCr := &hppv1.HostPathProvisioner{}
		err = r.client.Get(context.TODO(), req.NamespacedName, updatedCr)
		Expect(err).NotTo(HaveOccurred())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionAvailable)).To(BeFalse())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
		Expect(conditions.IsStatusConditionTrue(updatedCr.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
		Expect(conditions.FindStatusCondition(updatedCr.Status.Conditions, conditions.ConditionDegraded).Message).To(Equal("Unable to successfully reconcile: create failed"))
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
	Expect(len(name)).To(BeNumerically(">=", maxMountNameLength))
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
		Expect(err).NotTo(HaveOccurred())
		ds.Status.NumberReady = 2
		ds.Status.DesiredNumberScheduled = 2
		err = cl.Status().Update(context.TODO(), ds)
		Expect(err).NotTo(HaveOccurred())
	}
	// Now make the csi daemonSet available, and reconcile again.
	dsCsi := &appsv1.DaemonSet{}
	dsNNCsi := types.NamespacedName{
		Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
	Expect(err).NotTo(HaveOccurred())

	dsCsi.Status.NumberReady = 2
	dsCsi.Status.DesiredNumberScheduled = 2
	err = cl.Status().Update(context.TODO(), dsCsi)
	Expect(err).NotTo(HaveOccurred())

	err = cl.Get(context.TODO(), dsNNCsi, dsCsi)
	Expect(err).NotTo(HaveOccurred())
	Expect(dsCsi.Status.NumberReady).To(Equal(int32(2)))
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
func verifyCreateDaemonSet(cl client.Client) {
	ds := &appsv1.DaemonSet{}
	nn := types.NamespacedName{
		Name:      MultiPurposeHostPathProvisionerName,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, ds)
	Expect(err).NotTo(HaveOccurred())
	// Check Service Account
	Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal(ProvisionerServiceAccountName))
	// Check k8s recommended labels
	Expect(ds.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
	Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(ProvisionerImageDefault))
	// No junk in .spec.selector, should only be a minimal set that is needed to know which pods are under our governance
	Expect(ds.Spec.Selector.MatchLabels).To(Equal(
		map[string]string{
			"k8s-app": MultiPurposeHostPathProvisionerName,
		},
	))
	// Check use naming prefix
	Expect(ds.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("false"))
	// Check directory
	Expect(ds.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal("/tmp/test"))
}

// Verify all the proper values are set when creating the daemonset
func verifyCreateDaemonSetCsi(cl client.Client) {
	ds := &appsv1.DaemonSet{}
	nn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, ds)
	Expect(err).NotTo(HaveOccurred())
	// Check Service Account
	Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal(ProvisionerServiceAccountNameCsi))
	// Check k8s recommended labels
	Expect(ds.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
	Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(CsiProvisionerImageDefault))
	// No junk in .spec.selector, should only be a minimal set that is needed to know which pods are under our governance
	Expect(ds.Spec.Selector.MatchLabels).To(Equal(
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
	Expect(err).NotTo(HaveOccurred())
	Expect(sa.ObjectMeta.Name).To(Equal(name))
	Expect(sa.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIClusterRole(cl client.Client, enableSnapshot bool) {
	crole := &rbacv1.ClusterRole{}
	nn := types.NamespacedName{
		Name: ProvisionerServiceAccountNameCsi,
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
	if enableSnapshot {
		expectedRules = append(expectedRules, createSnapshotCsiClusterRoles()...)
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
		Name:      ProvisionerServiceAccountNameCsi,
		Namespace: testNamespace,
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
		Name: ProvisionerServiceAccountNameCsi,
	}
	err := cl.Get(context.TODO(), nn, crb)
	Expect(err).NotTo(HaveOccurred())
	Expect(crb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountNameCsi))
	Expect(crb.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateCSIRoleBinding(cl client.Client) {
	rb := &rbacv1.RoleBinding{}
	nn := types.NamespacedName{
		Name:      ProvisionerServiceAccountNameCsi,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, rb)
	Expect(err).NotTo(HaveOccurred())
	Expect(rb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountNameCsi))
	Expect(rb.Subjects[0].Namespace).To(Equal(testNamespace))
	Expect(rb.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreateSCC(cl client.Client) {
	scc := &secv1.SecurityContextConstraints{}
	nn := types.NamespacedName{
		Name: fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
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
	Expect(scc).To(Equal(expected))
	Expect(scc.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
}

func verifyCreatePrometheusResources(cl client.Client) {
	// PrometheusRule
	rule := &promv1.PrometheusRule{}
	nn := types.NamespacedName{
		Name:      ruleName,
		Namespace: testNamespace,
	}
	err := cl.Get(context.TODO(), nn, rule)
	Expect(err).NotTo(HaveOccurred())
	for _, r := range rule.Spec.Groups[0].Rules {
		if r.Record != "" {
			Expect(r.Annotations).To(BeNil())
		}
	}

	runbookURLTemplate := getRunbookURLTemplate()

	hppDownAlert := promv1.Rule{
		Alert: "HPPOperatorDown",
		Expr:  intstr.FromString("kubevirt_hpp_operator_up_total == 0"),
		For:   "5m",
		Annotations: map[string]string{
			"summary":     "Hostpath Provisioner operator is down",
			"runbook_url": fmt.Sprintf(runbookURLTemplate, "HPPOperatorDown"),
		},
		Labels: map[string]string{
			"severity":                      "warning",
			"operator_health_impact":        "critical",
			"kubernetes_operator_part_of":   "kubevirt",
			"kubernetes_operator_component": "hostpath-provisioner-operator",
		},
	}
	Expect(rule.Spec.Groups[0].Rules).To(ContainElement(hppDownAlert))
	Expect(rule.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))

	// ServiceMonitor
	monitor := &promv1.ServiceMonitor{}
	nn = types.NamespacedName{
		Name:      monitorName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, monitor)
	Expect(err).NotTo(HaveOccurred())
	Expect(monitor.Spec.NamespaceSelector.MatchNames).To(ContainElement(testNamespace))
	Expect(rule.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))

	// RBAC
	role := &rbacv1.Role{}
	roleBinding := &rbacv1.RoleBinding{}
	nn = types.NamespacedName{
		Name:      rbacName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, role)
	Expect(err).NotTo(HaveOccurred())
	Expect(role.Rules[0].Resources).To(ContainElement("endpoints"))
	Expect(rule.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
	err = cl.Get(context.TODO(), nn, roleBinding)
	Expect(err).NotTo(HaveOccurred())
	Expect(roleBinding.Subjects[0].Name).To(Equal("prometheus-k8s"))
	Expect(roleBinding.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))

	// Service
	service := &corev1.Service{}
	nn = types.NamespacedName{
		Name:      PrometheusServiceName,
		Namespace: testNamespace,
	}
	err = cl.Get(context.TODO(), nn, service)
	Expect(err).NotTo(HaveOccurred())
	port := corev1.ServicePort{
		Name: "metrics",
		Port: 8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "metrics",
		},
		Protocol: corev1.ProtocolTCP,
	}
	Expect(service.Spec.Ports).To(ContainElement(port))
	Expect(service.Spec.Selector).To(Equal(map[string]string{PrometheusLabelKey: PrometheusLabelValue}))
	Expect(service.Labels[AppKubernetesPartOfLabel]).To(Equal("testing"))
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
	Expect(len(cd.Spec.VolumeLifecycleModes)).To(Equal(2))
	Expect(cd.Spec.VolumeLifecycleModes).To(ContainElements(storagev1.VolumeLifecyclePersistent, storagev1.VolumeLifecycleEphemeral))
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
