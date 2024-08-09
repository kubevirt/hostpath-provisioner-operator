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
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

const (
	legacyDataDir                    = "csi-data-dir"
	legacyStoragePoolDataDir         = "legacy-data-dir"
	localDataDir                     = "local-data-dir"
	hashedLongStoragePoolNameDataDir = "l12345678901234567890123456789012345678901234-69d4290d-data-dir"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("daemonset", func() {
		var (
			cl client.Client
			r  *ReconcileHostPathProvisioner
		)
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() string {
				return testNamespace
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.It("Should properly generate the datadir for legacy CR", func() {
			_, _, cl := createDeployedCr(createLegacyCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			foundMount := false
			for _, container := range ds.Spec.Template.Spec.Containers {
				if container.Name == MultiPurposeHostPathProvisionerName {
					for _, mount := range container.VolumeMounts {
						if mount.MountPath == "/csi-data-dir" {
							gomega.Expect(mount.Name).To(gomega.Equal(legacyDataDir))
							foundMount = true
						}
					}
				}
			}
			gomega.Expect(foundMount).To(gomega.BeTrue(), "did not find expected volume mount named csi-data-dir")
			foundVolume := false
			for _, volume := range ds.Spec.Template.Spec.Volumes {
				log.Info("Volume", "name", volume.Name)
				if volume.Name == legacyDataDir {
					gomega.Expect(volume.HostPath).NotTo(gomega.BeNil())
					gomega.Expect(volume.HostPath.Path).To(gomega.Equal("/tmp/test"))
					foundVolume = true
				}
			}
			gomega.Expect(foundVolume).To(gomega.BeTrue(), "did not find expected volume named csi-data-dir")
		})

		ginkgo.It("Should properly generate the datadir for volumesource CR", func() {
			_, _, cl := createDeployedCr(createLegacyStoragePoolCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			foundMount := false
			for _, container := range ds.Spec.Template.Spec.Containers {
				if container.Name == MultiPurposeHostPathProvisionerName {
					for _, mount := range container.VolumeMounts {
						if mount.MountPath == "/legacy-data-dir" {
							gomega.Expect(mount.Name).To(gomega.Equal(legacyStoragePoolDataDir))
							foundMount = true
						}
					}
				}
			}
			gomega.Expect(foundMount).To(gomega.BeTrue(), "did not find expected volume mount named legacy-data-dir")
			foundVolume := false
			for _, volume := range ds.Spec.Template.Spec.Volumes {
				log.Info("Volume", "name", volume.Name)
				if volume.Name == legacyStoragePoolDataDir {
					gomega.Expect(volume.HostPath).NotTo(gomega.BeNil())
					gomega.Expect(volume.HostPath.Path).To(gomega.Equal("/tmp/test"))
					foundVolume = true
				}
			}
			gomega.Expect(foundVolume).To(gomega.BeTrue(), "did not find expected volume path /tmp/test")
		})

		ginkgo.It("Should fix a changed legacy daemonSet", func() {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, r, cl = createDeployedCr(createLegacyCr())
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      MultiPurposeHostPathProvisionerName,
					Namespace: testNamespace,
				},
			}
			dsNN := client.ObjectKeyFromObject(ds)
			err := cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds.Spec.Template.Spec.Volumes[0].Name = "invalid"
			err = cl.Update(context.TODO(), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds = &appsv1.DaemonSet{}
			err = cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(gomega.Equal("invalid"))

			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())
			// Check the daemonSet value, make sure it changed back.
			ds = &appsv1.DaemonSet{}
			err = cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(gomega.Equal(legacyVolume))
		})

		ginkgo.DescribeTable("Should fix a changed csi daemonSet", func(cr *hppv1.HostPathProvisioner, volumeMountName string) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl = createDeployedCr(cr)
			// Now modify the daemonSet to something not desired.
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			dsNN := client.ObjectKeyFromObject(ds)
			err := cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds.Spec.Template.Spec.Volumes[0].Name = "invalid"
			err = cl.Update(context.TODO(), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds = &appsv1.DaemonSet{}
			err = cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(gomega.Equal("invalid"))

			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())
			// Check the daemonSet value, make sure it changed back.
			ds = &appsv1.DaemonSet{}
			err = cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Template.Spec.Volumes[0].Name).To(gomega.Equal(socketDir))
			foundVolumeMountName := false
			volumeMounts := make([]string, 0)
			for _, container := range ds.Spec.Template.Spec.Containers {
				for _, volumeMount := range container.VolumeMounts {
					volumeMounts = append(volumeMounts, volumeMount.Name)
					if strings.Contains(volumeMount.Name, volumeMountName) {
						foundVolumeMountName = true
					}
				}
			}
			gomega.Expect(foundVolumeMountName).To(gomega.BeTrue(), fmt.Sprintf("Did not find volumeMount with string %s, in %v", volumeMountName, volumeMounts))
		},
			ginkgo.Entry("legacyCr", createLegacyCr(), legacyDataDir),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr(), legacyStoragePoolDataDir),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr(), localDataDir),
			ginkgo.Entry("longNamecr", createStoragePoolWithTemplateLongNameCr(), hashedLongStoragePoolNameDataDir),
		)

		ginkgo.DescribeTable("DaemonSet should have prometheus labels, port", func(cr *hppv1.HostPathProvisioner) {
			_, r, cl = createDeployedCr(cr)
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
					Namespace: testNamespace,
				},
			}
			dsNN := client.ObjectKeyFromObject(ds)
			err := cl.Get(context.TODO(), dsNN, ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			port := corev1.ContainerPort{
				ContainerPort: 8080,
				Name:          "metrics",
				Protocol:      corev1.ProtocolTCP,
			}
			gomega.Expect(ds.Spec.Template.Spec.Containers[0].Ports).To(gomega.ContainElement(port))
			gomega.Expect(ds.Labels[PrometheusLabelKey]).To(gomega.Equal(PrometheusLabelValue))
			gomega.Expect(ds.Spec.Template.Labels[PrometheusLabelKey]).To(gomega.Equal(PrometheusLabelValue))
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		ginkgo.DescribeTable("Should create daemonset with node placement", func(dsName string) {
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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(ds.Spec.Template.Spec.Affinity).To(gomega.BeNil())
			gomega.Expect(ds.Spec.Template.Spec.NodeSelector).To(gomega.BeEmpty())
			gomega.Expect(ds.Spec.Template.Spec.Tolerations).To(gomega.BeEmpty())

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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			cr.Spec.Workload = hppv1.NodePlacement{
				NodeSelector: nodeSelectorTestValue,
				Affinity:     affinityTestValue,
				Tolerations:  tolerationsTestValue,
			}
			err = cl.Update(context.TODO(), cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())

			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(ds.Spec.Template.Spec.Affinity).To(gomega.Equal(affinityTestValue))
			gomega.Expect(ds.Spec.Template.Spec.NodeSelector).To(gomega.Equal(nodeSelectorTestValue))
			gomega.Expect(ds.Spec.Template.Spec.Tolerations).To(gomega.Equal(tolerationsTestValue))
		},
			ginkgo.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			ginkgo.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)

		ginkgo.DescribeTable("Should be able to remove node placement if CR doesn't have it anymore", func(dsName string) {
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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(ds.Spec.Template.Spec.Affinity).To(gomega.Equal(affinityTestValue))
			gomega.Expect(ds.Spec.Template.Spec.NodeSelector).To(gomega.Equal(nodeSelectorTestValue))
			gomega.Expect(ds.Spec.Template.Spec.Tolerations).To(gomega.Equal(tolerationsTestValue))

			cr = &hppv1.HostPathProvisioner{}
			err = cl.Get(context.TODO(), req.NamespacedName, cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			cr.Spec.Workload = hppv1.NodePlacement{}
			err = cl.Update(context.TODO(), cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())

			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(ds.Spec.Template.Spec.Affinity).To(gomega.BeNil())
			gomega.Expect(ds.Spec.Template.Spec.NodeSelector).To(gomega.BeEmpty())
			gomega.Expect(ds.Spec.Template.Spec.Tolerations).To(gomega.BeEmpty())
		},
			ginkgo.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			ginkgo.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)

		ginkgo.DescribeTable("Should delete daemonsets from versions with junk in .spec.selector", func(dsName string) {
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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds.Spec.Selector.MatchLabels = map[string]string{
				"k8s-app": MultiPurposeHostPathProvisionerName,
				"not":     "desired",
			}
			err = cl.Update(context.TODO(), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Selector.MatchLabels).To(gomega.Equal(
				map[string]string{
					"k8s-app": MultiPurposeHostPathProvisionerName,
					"not":     "desired",
				},
			))

			// Run the reconcile loop
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.Equal("DaemonSet with extra selector labels spotted, cleaning up and requeueing"))
			// Artificial requeue (err occured implies requeue)
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Check the daemonSet value, make sure it changed back.
			ds = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: testNamespace,
				},
			}
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ds.Spec.Selector.MatchLabels).To(gomega.Equal(selectorLabels))
		},
			ginkgo.Entry("legacyDs", MultiPurposeHostPathProvisionerName),
			ginkgo.Entry("csiDs", fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)
	})
})
