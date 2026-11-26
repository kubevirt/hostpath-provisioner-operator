/*
Copyright 2026 The hostpath provisioner operator Authors.

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
	"encoding/json"
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

func testResources(cpuRequest, memoryRequest, cpuLimit, memoryLimit string) *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuRequest),
			corev1.ResourceMemory: resource.MustParse(memoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLimit),
			corev1.ResourceMemory: resource.MustParse(memoryLimit),
		},
	}
}

func testComponentResources() *hppv1.ComponentResources {
	return &hppv1.ComponentResources{
		HostPathProvisioner: testResources("100m", "200Mi", "1", "1Gi"),
		CSIProvisioner:      testResources("101m", "201Mi", "2", "2Gi"),
		NodeDriverRegistrar: testResources("102m", "202Mi", "3", "3Gi"),
		LivenessProbe:       testResources("103m", "203Mi", "4", "4Gi"),
		CSISnapshotter:      testResources("104m", "204Mi", "5", "5Gi"),
		Mounter:             testResources("105m", "205Mi", "6", "6Gi"),
	}
}

func getDaemonSet(cl client.Client, name string) *appsv1.DaemonSet {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	return ds
}

func containerResourcesByName(ds *appsv1.DaemonSet) map[string]corev1.ResourceRequirements {
	res := make(map[string]corev1.ResourceRequirements)
	for _, container := range ds.Spec.Template.Spec.Containers {
		res[container.Name] = container.Resources
	}
	return res
}

func updateCrSpec(cl client.Client, r *ReconcileHostPathProvisioner, mutate func(cr *hppv1.HostPathProvisioner)) {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: testNamespace,
		},
	}
	cr := &hppv1.HostPathProvisioner{}
	err := cl.Get(context.TODO(), req.NamespacedName, cr)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	mutate(cr)
	err = cl.Update(context.TODO(), cr)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	_, err = r.Reconcile(context.TODO(), req)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
}

var _ = ginkgo.Describe("Container resources", func() {
	csiDsName := fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)

	ginkgo.BeforeEach(func() {
		watchNamespaceFunc = func() string {
			return testNamespace
		}
		version.VersionStringFunc = func() (string, error) {
			return versionString, nil
		}
	})

	ginkgo.Context("getContainerResources", func() {
		ginkgo.It("Should return the defaults if no resources are specified", func() {
			cr := createLegacyCr()
			gomega.Expect(getContainerResources(cr, csiProvisionerResources, defaultContainerResources())).To(gomega.Equal(defaultContainerResources()))
			cr.Spec.Resources = &hppv1.ComponentResources{}
			gomega.Expect(getContainerResources(cr, csiProvisionerResources, defaultContainerResources())).To(gomega.Equal(defaultContainerResources()))
			gomega.Expect(getContainerResources(cr, mounterResources, defaultMounterResources())).To(gomega.Equal(defaultMounterResources()))
		})

		ginkgo.It("Should replace the defaults as a whole when resources are specified", func() {
			cr := createLegacyCr()
			cr.Spec.Resources = &hppv1.ComponentResources{
				CSIProvisioner: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("300Mi"),
					},
				},
			}
			res := getContainerResources(cr, csiProvisionerResources, defaultContainerResources())
			gomega.Expect(res.Requests).To(gomega.BeNil())
			gomega.Expect(res.Limits).To(gomega.Equal(corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("300Mi")}))
			// Other containers keep their defaults
			gomega.Expect(getContainerResources(cr, livenessProbeResources, defaultContainerResources())).To(gomega.Equal(defaultContainerResources()))
		})

		ginkgo.It("Should allow removing the default requests with an empty value", func() {
			cr := createLegacyCr()
			cr.Spec.Resources = &hppv1.ComponentResources{
				LivenessProbe: &corev1.ResourceRequirements{},
			}
			gomega.Expect(getContainerResources(cr, livenessProbeResources, defaultContainerResources())).To(gomega.Equal(corev1.ResourceRequirements{}))
		})

		ginkgo.It("Should canonicalize quantities so they match what the API server returns", func() {
			cr := createLegacyCr()
			cr.Spec.Resources = &hppv1.ComponentResources{}
			gomega.Expect(json.Unmarshal([]byte(`{"hostPathProvisioner":{"requests":{"cpu":"0.5","memory":"0.5Gi"}}}`), cr.Spec.Resources)).To(gomega.Succeed())
			res := getContainerResources(cr, provisionerResources, defaultContainerResources())
			// Round trip through JSON like the API server would.
			bytes, err := json.Marshal(res)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(bytes)).To(gomega.Equal(`{"requests":{"cpu":"500m","memory":"512Mi"}}`))
			roundTripped := corev1.ResourceRequirements{}
			gomega.Expect(json.Unmarshal(bytes, &roundTripped)).To(gomega.Succeed())
			gomega.Expect(res).To(gomega.Equal(roundTripped))
		})
	})

	ginkgo.Context("daemonsets", func() {
		ginkgo.It("Should use the default resources when none are specified", func() {
			_, r, cl := createDeployedCr(createLegacyCr())
			updateCrSpec(cl, r, func(cr *hppv1.HostPathProvisioner) {
				cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
			})
			csiResources := containerResourcesByName(getDaemonSet(cl, csiDsName))
			gomega.Expect(csiResources).To(gomega.HaveLen(5))
			for _, name := range []string{MultiPurposeHostPathProvisionerName, "csi-provisioner", nodeDriverRegistrarName, "liveness-probe"} {
				gomega.Expect(csiResources[name]).To(gomega.Equal(defaultContainerResources()), name)
			}
			gomega.Expect(csiResources["csi-snapshotter"]).To(gomega.Equal(corev1.ResourceRequirements{}))
			legacyResources := containerResourcesByName(getDaemonSet(cl, MultiPurposeHostPathProvisionerName))
			gomega.Expect(legacyResources[MultiPurposeHostPathProvisionerName]).To(gomega.Equal(defaultContainerResources()))
		})

		ginkgo.It("Should use the resources specified in the CR, and revert to defaults when removed", func() {
			cr := createLegacyCr()
			cr.Spec.Resources = testComponentResources()
			_, r, cl := createDeployedCr(cr)
			updateCrSpec(cl, r, func(cr *hppv1.HostPathProvisioner) {
				cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
			})
			expected := testComponentResources()
			csiResources := containerResourcesByName(getDaemonSet(cl, csiDsName))
			gomega.Expect(csiResources).To(gomega.HaveLen(5))
			gomega.Expect(csiResources[MultiPurposeHostPathProvisionerName]).To(gomega.Equal(*expected.HostPathProvisioner))
			gomega.Expect(csiResources["csi-provisioner"]).To(gomega.Equal(*expected.CSIProvisioner))
			gomega.Expect(csiResources[nodeDriverRegistrarName]).To(gomega.Equal(*expected.NodeDriverRegistrar))
			gomega.Expect(csiResources["liveness-probe"]).To(gomega.Equal(*expected.LivenessProbe))
			gomega.Expect(csiResources["csi-snapshotter"]).To(gomega.Equal(*expected.CSISnapshotter))
			legacyResources := containerResourcesByName(getDaemonSet(cl, MultiPurposeHostPathProvisionerName))
			gomega.Expect(legacyResources[MultiPurposeHostPathProvisionerName]).To(gomega.Equal(*expected.HostPathProvisioner))

			ginkgo.By("Changing a single container's resources")
			updateCrSpec(cl, r, func(cr *hppv1.HostPathProvisioner) {
				cr.Spec.Resources.CSIProvisioner = testResources("50m", "50Mi", "500m", "500Mi")
			})
			csiResources = containerResourcesByName(getDaemonSet(cl, csiDsName))
			gomega.Expect(csiResources["csi-provisioner"]).To(gomega.Equal(*testResources("50m", "50Mi", "500m", "500Mi")))
			gomega.Expect(csiResources[nodeDriverRegistrarName]).To(gomega.Equal(*expected.NodeDriverRegistrar))

			ginkgo.By("Removing the resources from the CR")
			updateCrSpec(cl, r, func(cr *hppv1.HostPathProvisioner) {
				cr.Spec.Resources = nil
			})
			csiResources = containerResourcesByName(getDaemonSet(cl, csiDsName))
			for _, name := range []string{MultiPurposeHostPathProvisionerName, "csi-provisioner", nodeDriverRegistrarName, "liveness-probe"} {
				gomega.Expect(csiResources[name]).To(gomega.Equal(defaultContainerResources()), name)
			}
			gomega.Expect(csiResources["csi-snapshotter"]).To(gomega.Equal(corev1.ResourceRequirements{}))
		})

		ginkgo.It("Should not update the daemonset on every reconcile when quantities are not canonical", func() {
			cr := createStoragePoolWithTemplateCr()
			cr.Spec.Resources = &hppv1.ComponentResources{
				CSIProvisioner: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("0.5"),
						corev1.ResourceMemory: resource.MustParse("0.5Gi"),
					},
				},
			}
			_, r, cl := createDeployedCr(cr)
			ds := getDaemonSet(cl, csiDsName)
			csiProvisionerRequests := containerResourcesByName(ds)["csi-provisioner"].Requests
			gomega.Expect(csiProvisionerRequests.Cpu().String()).To(gomega.Equal("500m"))
			resourceVersion := ds.ResourceVersion
			_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-name", Namespace: testNamespace}})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(getDaemonSet(cl, csiDsName).ResourceVersion).To(gomega.Equal(resourceVersion))
		})
	})

	ginkgo.Context("storage pool mounter", func() {
		ginkgo.It("Should use the default mounter resources when none are specified", func() {
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
			scaleClusterNodesAndDsUp(1, 1, cr, r, cl)
			deployment := &appsv1.Deployment{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: "hpp-pool-local-node1", Namespace: testNamespace}, deployment)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(gomega.Equal(defaultMounterResources()))
		})

		ginkgo.It("Should use the mounter resources specified in the CR in deployments and cleanup jobs", func() {
			cr := createStoragePoolWithTemplateCr()
			cr.Spec.Resources = testComponentResources()
			cr, r, cl := createDeployedCr(cr)
			scaleClusterNodesAndDsUp(1, 1, cr, r, cl)
			expected := *testComponentResources().Mounter
			deployment := &appsv1.Deployment{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: "hpp-pool-local-node1", Namespace: testNamespace}, deployment)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(gomega.Equal(expected))

			ginkgo.By("Marking CR as deleted, the cleanup job should use the mounter resources")
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = cl.Delete(context.TODO(), cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-name", Namespace: testNamespace}})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			jobList := &batchv1.JobList{}
			err = cl.List(context.TODO(), jobList)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(jobList.Items).To(gomega.HaveLen(1))
			gomega.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Resources).To(gomega.Equal(expected))
		})
	})
})
