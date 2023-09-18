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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("storage pool", func() {
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.DescribeTable("Should not create a storage pool for legacy crs", func(cr *hppv1.HostPathProvisioner) {
			cr, _, cl := createDeployedCr(cr)
			deploymentList := &appsv1.DeploymentList{}

			cl.List(context.TODO(), deploymentList, &client.ListOptions{
				Namespace: testNamespace,
			})
			foundDeployments := make([]string, 0)
			for _, deployment := range deploymentList.Items {
				if metav1.IsControlledBy(&deployment, cr) {
					foundDeployments = append(foundDeployments, deployment.Name)
				}
			}
			gomega.Expect(foundDeployments).To(gomega.BeEmpty(), fmt.Sprintf("%v", foundDeployments))
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
		)

		ginkgo.DescribeTable("Should create a storage pool for a storage pool with template", func(cr *hppv1.HostPathProvisioner) {
			cr, r, cl := createDeployedCr(cr)
			scaleClusterNodesAndDsUp(1, 10, cr, r, cl)
			// Expect 10 pods and 10 pvcs
			verifyDeploymentsAndPVCs(10, 10, cr, r, cl)
		},
			ginkgo.Entry("filesystem", createStoragePoolWithTemplateCr()),
			ginkgo.Entry("filesystem long name", createStoragePoolWithTemplateLongNameCr()),
			ginkgo.Entry("block", createStoragePoolWithTemplateBlockCr()),
		)

		ginkgo.It("Should scale the deployments and pvcs, if daemonset scales", func() {
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
			scaleClusterNodesAndDsUp(1, 6, cr, r, cl)
			// Expect 6 pods and 6 pvcs
			verifyDeploymentsAndPVCs(6, 6, cr, r, cl)
			scaleClusterNodesAndDsUp(7, 10, cr, r, cl)
			// Expect 10 pods and 10 pvcs
			verifyDeploymentsAndPVCs(10, 10, cr, r, cl)
			scaleClusterNodesAndDsDown(3, 8, 4, cr, r, cl)
			// Expect 4 pods and 10 pvcs, since pvcs don't get deleted.
			verifyDeploymentsAndPVCs(4, 10, cr, r, cl)
		})

		ginkgo.It("Should fix modified storage pool deployments", func() {
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
			scaleClusterNodesAndDsUp(1, 1, cr, r, cl)
			// Expect 1 pods and 1 pvcs
			verifyDeploymentsAndPVCs(1, 1, cr, r, cl)
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("hpp-pool-%s-%s", "local", "node1"),
					Namespace: testNamespace,
				},
			}
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deployment.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
			gomega.Expect(deployment.Spec.Template.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
			deployment.Spec.Template.Spec.Containers[0].Name = "failure"
			cl.Update(context.TODO(), deployment)
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(gomega.Equal("failure"))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(gomega.Equal("mounter"))
		})

		ginkgo.It("Should create cleanup jobs, if CR is marked for deletion", func() {
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
			scaleClusterNodesAndDsUp(1, 1, cr, r, cl)
			// Expect 1 pods and 1 pvcs
			verifyDeploymentsAndPVCs(1, 1, cr, r, cl)

			ginkgo.By("Marking CR as deleted, it should generate cleanup jobs after reconcile")
			err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			err = cl.Delete(context.TODO(), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			jobList := &batchv1.JobList{}
			err = r.client.List(context.TODO(), jobList)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(len(jobList.Items)).To(gomega.Equal(1))
			gomega.Expect(jobList.Items[0].GetName()).To(gomega.Equal("cleanup-pool-local-node1"))
			gomega.Expect(jobList.Items[0].Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
			gomega.Expect(jobList.Items[0].Spec.Template.Labels[AppKubernetesPartOfLabel]).To(gomega.Equal("testing"))
		})

		ginkgo.It("Status length should remain at one with legacy CR", func() {
			cr, r, cl := createDeployedCr(createLegacyCr())
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(len(cr.Status.StoragePoolStatuses)).To(gomega.Equal(1))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			for i := 0; i < 10; i++ {
				_, err := r.Reconcile(context.TODO(), req)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			gomega.Expect(len(cr.Status.StoragePoolStatuses)).To(gomega.Equal(1))
		})

		ginkgo.It("should allow creation and deletion of mixed CR", func() {
			blockMode := corev1.PersistentVolumeBlock
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateVolumeModeAndBasicCr("template", &blockMode))
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(len(cr.Status.StoragePoolStatuses)).To(gomega.Equal(2))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			gomega.Expect(len(cr.Status.StoragePoolStatuses)).To(gomega.Equal(2))
			scaleClusterNodesAndDsUp(1, 6, cr, r, cl)
			deployments := appsv1.DeploymentList{}
			err = cl.List(context.TODO(), &deployments)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deployments.Items).ToNot(gomega.BeEmpty())
			for _, deployment := range deployments.Items {
				ginkgo.By("Setting deployment " + deployment.Name + " replicas to 1")
				deployment.Status.ReadyReplicas = int32(1)
				err = cl.Status().Update(context.TODO(), &deployment)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Delete the CR.
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			cr.Finalizers = append(cr.Finalizers, "test")
			err = cl.Delete(context.TODO(), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = r.Reconcile(context.TODO(), req)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			deployments = appsv1.DeploymentList{}
			err = cl.List(context.TODO(), &deployments)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deployments.Items).To(gomega.BeEmpty())
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			cr.Finalizers = nil
			err = cl.Update(context.TODO(), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
})

func scaleClusterNodesAndDsUp(start, end int, cr *hppv1.HostPathProvisioner, r *ReconcileHostPathProvisioner, cl client.Client) {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: testNamespace,
		},
	}
	addNodesToCluster(start, end, cl)
	ginkgo.By("Finding the csi daemonset, and create pods for each node")
	csiDs := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(csiDs), csiDs)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	csiDs.Status.DesiredNumberScheduled = int32(end)
	csiDs.Status.NumberAvailable = int32(end)
	csiDs.Status.NumberReady = int32(end)
	err = cl.Status().Update(context.TODO(), csiDs)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	createCsiDsPods(start, end, csiDs, cl)

	ginkgo.By("reconciling again, it should create storage pools")
	_, err = r.Reconcile(context.TODO(), req)
	gomega.Expect(err).To(gomega.HaveOccurred())
	gomega.Expect(err.Error()).To(gomega.ContainSubstring("instead of Bound"))
	err = r.client.Get(context.TODO(), req.NamespacedName, cr)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(IsHppAvailable(cr)).To(gomega.BeFalse())
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = cl.List(context.TODO(), pvcList, &client.ListOptions{
		Namespace: testNamespace,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	for _, pvc := range pvcList.Items {
		pvc.Status.Phase = corev1.ClaimBound
		err = cl.Status().Update(context.TODO(), &pvc)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	_, err = r.Reconcile(context.TODO(), req)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = r.client.Get(context.TODO(), req.NamespacedName, cr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(IsHppAvailable(cr)).To(gomega.BeTrue())
}

func scaleClusterNodesAndDsDown(start, end, newCount int, _ *hppv1.HostPathProvisioner, r *ReconcileHostPathProvisioner, cl client.Client) {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: testNamespace,
		},
	}
	removeNodesFromCluster(start, end, cl)
	ginkgo.By("Finding the csi daemonset, and create pods for each node")
	csiDs := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(csiDs), csiDs)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	csiDs.Status.DesiredNumberScheduled = int32(newCount)
	csiDs.Status.NumberAvailable = int32(newCount)
	csiDs.Status.NumberReady = int32(newCount)
	err = cl.Status().Update(context.TODO(), csiDs)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	deleteCsiDsPods(start, end, csiDs, cl)

	ginkgo.By("reconciling again, it should create storage pools")
	_, err = r.Reconcile(context.TODO(), req)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

}

func verifyDeploymentsAndPVCs(podCount, pvcCount int, cr *hppv1.HostPathProvisioner, _ *ReconcileHostPathProvisioner, cl client.Client) {
	deploymentList := &appsv1.DeploymentList{}
	err := cl.List(context.TODO(), deploymentList, &client.ListOptions{
		Namespace: testNamespace,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	storagePoolName := ""
	for _, storagePool := range cr.Spec.StoragePools {
		storagePoolName = storagePool.Name
	}
	gomega.Expect(storagePoolName).ToNot(gomega.BeEmpty())
	foundDeployments := make([]string, 0)
	for _, deployment := range deploymentList.Items {
		if metav1.IsControlledBy(&deployment, cr) && deployment.GetLabels()[storagePoolLabelKey] == getResourceNameWithMaxLength(storagePoolName, "hpp", maxNameLength) {
			foundDeployments = append(foundDeployments, deployment.Name)
		}
		gomega.Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(gomega.Equal(ProvisionerServiceAccountNameCsi))
	}
	gomega.Expect(foundDeployments).ToNot(gomega.BeEmpty(), fmt.Sprintf("%v", foundDeployments))
	gomega.Expect(len(foundDeployments)).To(gomega.Equal(podCount), fmt.Sprintf("%v", foundDeployments))

	pvcList := &corev1.PersistentVolumeClaimList{}
	foundPVCs := make([]string, 0)
	err = cl.List(context.TODO(), pvcList, &client.ListOptions{
		Namespace: testNamespace,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	pvcNames := make([]string, 0)
	for i := 1; i <= pvcCount; i++ {
		pvcNames = append(pvcNames, getStoragePoolPVCName(storagePoolName, fmt.Sprintf("node%d", i)))
	}
	for _, pvc := range pvcList.Items {
		foundPVCs = append(foundPVCs, pvc.Name)
		gomega.Expect(pvc.GetLabels()[storagePoolLabelKey]).To(gomega.Equal(getResourceNameWithMaxLength(storagePoolName, "hpp", maxNameLength)))
		gomega.Expect(pvcNames).To(gomega.ContainElement(pvc.Name))
		gomega.Expect(pvc.Spec).To(gomega.BeEquivalentTo(*cr.Spec.StoragePools[0].PVCTemplate))
	}
	gomega.Expect(foundPVCs).ToNot(gomega.BeEmpty(), fmt.Sprintf("%v", foundPVCs))
	gomega.Expect(len(foundPVCs)).To(gomega.Equal(pvcCount))
}

func addNodesToCluster(start, end int, cl client.Client) {
	for i := start; i <= end; i++ {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("node%d", i),
			},
		}
		err := cl.Create(context.TODO(), node)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func removeNodesFromCluster(start, end int, cl client.Client) {
	for i := start; i <= end; i++ {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("node%d", i),
			},
		}
		err := cl.Delete(context.TODO(), node)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func createCsiDsPods(start, end int, ds *appsv1.DaemonSet, cl client.Client) {
	controller := true
	for i := start; i <= end; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod%d", i),
				Namespace: testNamespace,
				Labels: map[string]string{
					"k8s-app": MultiPurposeHostPathProvisionerName,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller: &controller,
						UID:        ds.GetUID(),
					},
				},
			},
			Spec: corev1.PodSpec{
				NodeName: fmt.Sprintf("node%d", i),
			},
		}
		err := cl.Create(context.TODO(), pod)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func deleteCsiDsPods(start, end int, _ *appsv1.DaemonSet, cl client.Client) {
	for i := start; i <= end; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod%d", i),
				Namespace: testNamespace,
				Labels: map[string]string{
					"k8s-app": MultiPurposeHostPathProvisionerName,
				},
			},
		}
		err := cl.Delete(context.TODO(), pod)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}
