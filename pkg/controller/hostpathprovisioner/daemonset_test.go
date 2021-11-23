/*
Copyright 2021 The hostpath provisioner operator Authors.

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller reconcile loop", func() {
	Context("daemonset", func() {
		var (
			cl client.Client
			r  *ReconcileHostPathProvisioner
		)
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		It("Should properly generate the datadir for legacy CR", func() {
			_, _, cl := createDeployedCr(createLegacyCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())
			foundMount := false
			for _, container := range ds.Spec.Template.Spec.Containers {
				if container.Name == MultiPurposeHostPathProvisionerName {
					for _, mount := range container.VolumeMounts {
						if mount.MountPath == "/csi-data-dir" {
							Expect(mount.Name).To(Equal("csi-data-dir"))
							foundMount = true
						}
					}
				}
			}
			Expect(foundMount).To(BeTrue(), "did not find expected volume mount named csi-data-dir")
			foundVolume := false
			for _, volume := range ds.Spec.Template.Spec.Volumes {
				log.Info("Volume", "name", volume.Name)
				if volume.Name == "csi-data-dir" {
					Expect(volume.HostPath).NotTo(BeNil())
					Expect(volume.HostPath.Path).To(Equal("/tmp/test"))
					foundVolume = true
				}
			}
			Expect(foundVolume).To(BeTrue(), "did not find expected volume named csi-data-dir")
		})

		It("Should properly generate the datadir for volumesource CR", func() {
			_, _, cl := createDeployedCr(createLegacyStoragePoolCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())
			foundMount := false
			for _, container := range ds.Spec.Template.Spec.Containers {
				if container.Name == MultiPurposeHostPathProvisionerName {
					for _, mount := range container.VolumeMounts {
						if mount.MountPath == "/legacy-data-dir" {
							Expect(mount.Name).To(Equal("legacy-data-dir"))
							foundMount = true
						}
					}
				}
			}
			Expect(foundMount).To(BeTrue(), "did not find expected volume mount named legacy-data-dir")
			foundVolume := false
			for _, volume := range ds.Spec.Template.Spec.Volumes {
				log.Info("Volume", "name", volume.Name)
				if volume.Name == "legacy-data-dir" {
					Expect(volume.HostPath).NotTo(BeNil())
					Expect(volume.HostPath.Path).To(Equal("/tmp/test"))
					foundVolume = true
				}
			}
			Expect(foundVolume).To(BeTrue(), "did not find expected volume path /tmp/test")
		})

		table.DescribeTable("Should fix a changed legacy daemonSet", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl = createDeployedCr(cr)
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: v1.ObjectMeta{
					Name:      MultiPurposeHostPathProvisionerName,
					Namespace: testNamespace,
				},
			}
			dsNN := client.ObjectKeyFromObject(ds)
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
			Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal(legacyVolume))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		table.DescribeTable("Should fix a changed csi daemonSet", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl = createDeployedCr(cr)
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: v1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			dsNN := client.ObjectKeyFromObject(ds)
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
			Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(Equal(socketDir))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		table.DescribeTable("Should create daemonset with node placement", func(dsName string) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl := createDeployedCr(createLegacyCr())
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}

			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
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

			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())

			Expect(ds.Spec.Template.Spec.Affinity).To(Equal(affinityTestValue))
			Expect(ds.Spec.Template.Spec.NodeSelector).To(Equal(nodeSelectorTestValue))
			Expect(ds.Spec.Template.Spec.Tolerations).To(Equal(tolerationsTestValue))
		},
			table.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			table.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)

		table.DescribeTable("Should be able to remove node placement if CR doesn't have it anymore", func(dsName string) {
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
			cr := createLegacyCr()
			cr.Spec.Workload = hppv1.NodePlacement{
				NodeSelector: nodeSelectorTestValue,
				Affinity:     affinityTestValue,
				Tolerations:  tolerationsTestValue,
			}
			cr, r, cl := createDeployedCr(cr)
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
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

			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())

			Expect(ds.Spec.Template.Spec.Affinity).To(BeNil())
			Expect(ds.Spec.Template.Spec.NodeSelector).To(BeEmpty())
			Expect(ds.Spec.Template.Spec.Tolerations).To(BeEmpty())
		},
			table.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			table.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)

		table.DescribeTable("Should delete daemonsets from versions with junk in .spec.selector", func(dsName string) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, r, cl := createDeployedCr(createLegacyCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())
			ds.Spec.Selector.MatchLabels = map[string]string{
				"k8s-app": MultiPurposeHostPathProvisionerName,
				"not":     "desired",
			}
			err = cl.Update(context.TODO(), ds)
			Expect(err).NotTo(HaveOccurred())
			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())
			Expect(ds.Spec.Selector.MatchLabels).To(Equal(
				map[string]string{
					"k8s-app": MultiPurposeHostPathProvisionerName,
					"not":     "desired",
				},
			))

			// Run the reconcile loop
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("DaemonSet with extra selector labels spotted, cleaning up and requeueing"))
			// Artificial requeue (err occured implies requeue)
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())
			// Check the daemonSet value, make sure it changed back.
			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			Expect(err).NotTo(HaveOccurred())
			Expect(ds.Spec.Selector.MatchLabels).To(Equal(selectorLabels))
		},
			table.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			table.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)
	})
})
