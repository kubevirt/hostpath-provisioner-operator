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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Context("storage pool", func() {
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		table.DescribeTable("Should not create a storage pool for legacy crs", func(cr *hppv1.HostPathProvisioner) {
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
			Expect(foundDeployments).To(BeEmpty(), fmt.Sprintf("%v", foundDeployments))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
		)

		table.DescribeTable("Should create a storage pool for a storage pool with template", func(cr *hppv1.HostPathProvisioner) {
			cr, r, cl := createDeployedCr(cr)
			scaleClusterNodesAndDsUp(1, 10, cr, r, cl)
			// Expect 10 pods and 10 pvcs
			verifyDeploymentsAndPVCs(10, 10, cr, r, cl)
		},
			table.Entry("filesystem", createStoragePoolWithTemplateCr()),
			table.Entry("filesystem long name", createStoragePoolWithTemplateLongNameCr()),
			table.Entry("block", createStoragePoolWithTemplateBlockCr()),
		)

		It("Should scale the deployments and pvcs, if daemonset scales", func() {
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

		It("Should fix modified storage pool deployments", func() {
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
			Expect(err).ToNot(HaveOccurred())
			deployment.Spec.Template.Spec.Containers[0].Name = "failure"
			cl.Update(context.TODO(), deployment)
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("failure"))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("mounter"))
		})

		It("Should create cleanup jobs, if CR is marked for deletion", func() {
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateCr())
			scaleClusterNodesAndDsUp(1, 1, cr, r, cl)
			// Expect 1 pods and 1 pvcs
			verifyDeploymentsAndPVCs(1, 1, cr, r, cl)
			deletionTime := metav1.NewTime(time.Now())

			By("Marking CR as deleted, it should generate cleanup jobs after reconcile")
			err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			Expect(err).ToNot(HaveOccurred())
			cr.DeletionTimestamp = &deletionTime
			err = cl.Update(context.TODO(), cr)
			Expect(err).ToNot(HaveOccurred())
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())
			jobList := &batchv1.JobList{}
			err = r.client.List(context.TODO(), jobList)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(jobList.Items)).To(Equal(1))
			Expect(jobList.Items[0].GetName()).To(Equal("cleanup-pool-local-node1"))
		})

		It("Status length should remain at one with legacy CR", func() {
			cr, r, cl := createDeployedCr(createLegacyCr())
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(cr.Status.StoragePoolStatuses)).To(Equal(1))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			for i := 0; i < 10; i++ {
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())
				err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(len(cr.Status.StoragePoolStatuses)).To(Equal(1))
		})

		It("should allow creation and deletion of mixed CR", func() {
			blockMode := corev1.PersistentVolumeBlock
			cr, r, cl := createDeployedCr(createStoragePoolWithTemplateVolumeModeAndBasicCr("template", &blockMode))
			err := cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(cr.Status.StoragePoolStatuses)).To(Equal(2))
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			Expect(len(cr.Status.StoragePoolStatuses)).To(Equal(2))
			scaleClusterNodesAndDsUp(1, 6, cr, r, cl)
			deployments := appsv1.DeploymentList{}
			err = cl.List(context.TODO(), &deployments)
			Expect(err).ToNot(HaveOccurred())
			Expect(deployments.Items).ToNot(BeEmpty())
			for _, deployment := range deployments.Items {
				By("Setting deployment " + deployment.Name + " replicas to 1")
				deployment.Status.ReadyReplicas = int32(1)
				err = cl.Update(context.TODO(), &deployment)
				Expect(err).ToNot(HaveOccurred())
			}
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())

			// Delete the CR.
			err = cl.Get(context.TODO(), client.ObjectKeyFromObject(cr), cr)
			Expect(err).ToNot(HaveOccurred())
			now := metav1.NewTime(time.Now())
			cr.DeletionTimestamp = &now
			time.Sleep(time.Millisecond)
			err = cl.Update(context.TODO(), cr)
			Expect(err).ToNot(HaveOccurred())
			_, err = r.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())
			deployments = appsv1.DeploymentList{}
			err = cl.List(context.TODO(), &deployments)
			Expect(err).ToNot(HaveOccurred())
			Expect(deployments.Items).To(BeEmpty())
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
	By("Finding the csi daemonset, and create pods for each node")
	csiDs := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(csiDs), csiDs)
	Expect(err).ToNot(HaveOccurred())
	csiDs.Status.DesiredNumberScheduled = int32(end)
	csiDs.Status.NumberAvailable = int32(end)
	csiDs.Status.NumberReady = int32(end)
	err = cl.Update(context.TODO(), csiDs)
	Expect(err).ToNot(HaveOccurred())
	createCsiDsPods(start, end, csiDs, cl)

	By("reconciling again, it should create storage pools")
	_, err = r.Reconcile(context.TODO(), req)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("instead of Bound"))
	err = r.client.Get(context.TODO(), req.NamespacedName, cr)
	Expect(err).ToNot(HaveOccurred())
	Expect(IsHppAvailable(cr)).To(BeFalse())
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = cl.List(context.TODO(), pvcList, &client.ListOptions{
		Namespace: testNamespace,
	})
	Expect(err).ToNot(HaveOccurred())
	for _, pvc := range pvcList.Items {
		pvc.Status.Phase = corev1.ClaimBound
		err = cl.Update(context.TODO(), &pvc)
		Expect(err).ToNot(HaveOccurred())
	}
	_, err = r.Reconcile(context.TODO(), req)
	Expect(err).ToNot(HaveOccurred())
	err = r.client.Get(context.TODO(), req.NamespacedName, cr)
	Expect(err).NotTo(HaveOccurred())
	Expect(IsHppAvailable(cr)).To(BeTrue())
}

func scaleClusterNodesAndDsDown(start, end, newCount int, cr *hppv1.HostPathProvisioner, r *ReconcileHostPathProvisioner, cl client.Client) {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: testNamespace,
		},
	}
	removeNodesFromCluster(start, end, cl)
	By("Finding the csi daemonset, and create pods for each node")
	csiDs := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName),
			Namespace: testNamespace,
		},
	}
	err := cl.Get(context.TODO(), client.ObjectKeyFromObject(csiDs), csiDs)
	Expect(err).ToNot(HaveOccurred())
	csiDs.Status.DesiredNumberScheduled = int32(newCount)
	csiDs.Status.NumberAvailable = int32(newCount)
	csiDs.Status.NumberReady = int32(newCount)
	err = cl.Update(context.TODO(), csiDs)
	Expect(err).ToNot(HaveOccurred())
	deleteCsiDsPods(start, end, csiDs, cl)

	By("reconciling again, it should create storage pools")
	_, err = r.Reconcile(context.TODO(), req)
	Expect(err).ToNot(HaveOccurred())

}

func verifyDeploymentsAndPVCs(podCount, pvcCount int, cr *hppv1.HostPathProvisioner, r *ReconcileHostPathProvisioner, cl client.Client) {
	deploymentList := &appsv1.DeploymentList{}
	err := cl.List(context.TODO(), deploymentList, &client.ListOptions{
		Namespace: testNamespace,
	})
	Expect(err).ToNot(HaveOccurred())
	storagePoolName := ""
	for _, storagePool := range cr.Spec.StoragePools {
		storagePoolName = storagePool.Name
	}
	Expect(storagePoolName).ToNot(BeEmpty())
	foundDeployments := make([]string, 0)
	for _, deployment := range deploymentList.Items {
		if metav1.IsControlledBy(&deployment, cr) && deployment.GetLabels()[storagePoolLabelKey] == getResourceNameWithMaxLength(storagePoolName, "hpp", maxNameLength) {
			foundDeployments = append(foundDeployments, deployment.Name)
		}
		Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(ProvisionerServiceAccountNameCsi))
	}
	Expect(foundDeployments).ToNot(BeEmpty(), fmt.Sprintf("%v", foundDeployments))
	Expect(len(foundDeployments)).To(Equal(podCount), fmt.Sprintf("%v", foundDeployments))

	pvcList := &corev1.PersistentVolumeClaimList{}
	foundPVCs := make([]string, 0)
	err = cl.List(context.TODO(), pvcList, &client.ListOptions{
		Namespace: testNamespace,
	})
	Expect(err).ToNot(HaveOccurred())
	pvcNames := make([]string, 0)
	for i := 1; i <= pvcCount; i++ {
		pvcNames = append(pvcNames, getStoragePoolPVCName(storagePoolName, fmt.Sprintf("node%d", i)))
	}
	for _, pvc := range pvcList.Items {
		foundPVCs = append(foundPVCs, pvc.Name)
		Expect(pvc.GetLabels()[storagePoolLabelKey]).To(Equal(getResourceNameWithMaxLength(storagePoolName, "hpp", maxNameLength)))
		Expect(pvcNames).To(ContainElement(pvc.Name))
		Expect(pvc.Spec).To(BeEquivalentTo(*cr.Spec.StoragePools[0].PVCTemplate))
	}
	Expect(foundPVCs).ToNot(BeEmpty(), fmt.Sprintf("%v", foundPVCs))
	Expect(len(foundPVCs)).To(Equal(pvcCount))
}

func addNodesToCluster(start, end int, cl client.Client) {
	for i := start; i <= end; i++ {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("node%d", i),
			},
		}
		err := cl.Create(context.TODO(), node)
		Expect(err).ToNot(HaveOccurred())
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
		Expect(err).ToNot(HaveOccurred())
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
		Expect(err).ToNot(HaveOccurred())
	}
}

func deleteCsiDsPods(start, end int, ds *appsv1.DaemonSet, cl client.Client) {
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
		Expect(err).ToNot(HaveOccurred())
	}
}
