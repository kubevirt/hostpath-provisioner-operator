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
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	storagePoolLabelKey     = "kubevirt.io.hostpath-provisioner/storagePool"
	dataName                = "data"
	fsDataMountPath         = "/source"
	blockDataMountPath      = "/dev/data"
	defaultStorageClassName = "default"
)

//StoragePoolInfo contains the name and path of a hostpath storage pool.
type StoragePoolInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (r *ReconcileHostPathProvisioner) reconcileStoragePools(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (reconcile.Result, error) {
	usedNodes, err := r.getNodesByDaemonSet(logger, namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	logger.Info("Checking if storage pools are configured", "current nodes number of used nodes", len(usedNodes))
	currentStoragePoolDeployments, err := r.currentStoragePoolDeployments(logger, cr, namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	for _, storagePool := range cr.Spec.StoragePools {
		logger.Info("Checking storage pool", "pool.Name", storagePool.Name)
		if storagePool.PVCTemplate != nil {
			for _, node := range usedNodes {
				if err := r.reconcileStoragePoolPVCByNode(logger, cr, namespace, &storagePool, &node); err != nil {
					return reconcile.Result{}, err
				}
				if err := r.reconcileStoragePoolDeploymentByNode(logger, cr, namespace, &storagePool, &node, currentStoragePoolDeployments); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}
	// Clean up any deployments that are no longer used.
	for _, ds := range currentStoragePoolDeployments {
		logger.Info("Deleting unused deployment", "deployment name", ds.GetName())
		if err := r.client.Delete(context.TODO(), &ds); err != nil && !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		sp := r.getStoragePoolForDeployment(cr, &ds)
		if sp != nil {
			if _, err := r.createCleanupJobForDeployment(logger, cr, namespace, &ds, sp); err != nil {

			}
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileHostPathProvisioner) getStoragePoolForDeployment(cr *hostpathprovisionerv1.HostPathProvisioner, deployment *appsv1.Deployment) *hostpathprovisionerv1.StoragePool {
	for _, storagePool := range cr.Spec.StoragePools {
		if strings.HasPrefix(deployment.GetName(), fmt.Sprintf("hpp-pool-%s", storagePool.Name)) {
			return &storagePool
		}
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) cleanDeployments(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) error {
	logger.Info("Cleaning up storage pools")
	for _, storagePool := range cr.Spec.StoragePools {
		currentStoragePoolDeployments, err := r.currentStoragePoolDeployments(logger, cr, namespace)
		if err != nil {
			return err
		}
		logger.Info("Cleanup up", "count", len(currentStoragePoolDeployments))
		for _, deployment := range currentStoragePoolDeployments {
			node, err := r.createCleanupJobForDeployment(logger, cr, namespace, &deployment, &storagePool)
			if err != nil {
				return err
			}
			desired := r.storagePoolDeploymentByNode(logger, cr, &storagePool, namespace, node)

			// delete deployment
			found := &appsv1.Deployment{}
			if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(desired), found); err != nil && !errors.IsNotFound(err) {
				return err
			} else if err == nil {
				if err := r.client.Delete(context.TODO(), found); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) createCleanupJobForDeployment(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, deployment *appsv1.Deployment, storagePool *hostpathprovisionerv1.StoragePool) (*corev1.Node, error) {
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0],
		},
	}
	if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(node), node); err != nil {
		return nil, err
	}
	logger.Info("for node", "name", node.Name)
	if err := r.createCleanupJobForNode(logger, cr, namespace, storagePool, node); err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}
	return node, nil
}

func (r *ReconcileHostPathProvisioner) reconcileStoragePoolPVCByNode(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, storagePool *hostpathprovisionerv1.StoragePool, node *corev1.Node) error {
	desired := r.storagePoolPVCByNode(storagePool, namespace, node)
	// Check if this SecurityContextConstraints already exists
	found := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(desired), found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new storage pool pvc on node", "storagepool.Name", storagePool.Name, "node.Name", node.GetName())
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.GetName(), err))
			return err
		}
		// PVC created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.GetName()))
	} else if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) reconcileStoragePoolDeploymentByNode(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, storagePool *hostpathprovisionerv1.StoragePool, node *corev1.Node, currentStoragePoolDeployments map[string]appsv1.Deployment) error {
	// Create stateful set that mounts the volume to the node
	desired := r.storagePoolDeploymentByNode(logger, cr, storagePool, namespace, node)
	setLastAppliedConfiguration(desired)
	// Set HostPathProvisioner instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, desired, r.scheme); err != nil {
		return err
	}

	// Check if this Deployment already exists
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(desired), found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new storage pool deployment on node", "storagepool.Name", storagePool.Name, "node.Name", node.GetName())
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf(createMessageFailed, desired.GetName(), err))
			return err
		}
		// Deployment created successfully - don't requeue
		r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf(createMessageSucceeded, desired, desired.GetName()))
		return nil
	} else if err != nil {
		return err
	}
	delete(currentStoragePoolDeployments, desired.GetName())

	// Keep a copy of the original for comparison later.
	currentRuntimeObjCopy := found.DeepCopyObject()

	// allow users to add new annotations (but not change ours)
	mergeLabelsAndAnnotations(desired, found)

	found.Spec = *desired.Spec.DeepCopy()

	if !reflect.DeepEqual(currentRuntimeObjCopy, found) {
		logJSONDiff(logger, currentRuntimeObjCopy, found)
		// Current is different from desired, update.
		logger.Info("Updating Deployment for node", "deployment.Name", desired.GetName(), "node.Name", node.GetName())
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf(updateMessageFailed, desired.GetName(), err))
			return err
		}
		r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf(updateMessageSucceeded, desired, desired.GetName()))
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) currentStoragePoolDeployments(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) (map[string]appsv1.Deployment, error) {
	res := make(map[string]appsv1.Deployment)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app": MultiPurposeHostPathProvisionerName,
		},
	})
	if err != nil {
		return res, err
	}
	deploymentList := &appsv1.DeploymentList{}
	r.client.List(context.TODO(), deploymentList, &client.ListOptions{
		LabelSelector: client.MatchingLabelsSelector{
			Selector: selector,
		},
	})
	for _, deployment := range deploymentList.Items {
		if metav1.IsControlledBy(&deployment, cr) {
			res[deployment.GetName()] = deployment
		}
	}

	return res, nil
}

func (r *ReconcileHostPathProvisioner) getNodesByDaemonSet(logger logr.Logger, namespace string) ([]corev1.Node, error) {
	res := make([]corev1.Node, 0)
	dsArgs := getDaemonSetArgs(logger, namespace, false)
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dsArgs.namespace,
			Name:      dsArgs.name,
		},
	}

	if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(ds), ds); err != nil {
		if errors.IsNotFound(err) {
			return res, nil
		}
		return res, err
	}
	logger.Info("Finding pods associated with daemonset", "daemonSet", ds.GetName())

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app": MultiPurposeHostPathProvisionerName,
		},
	})
	if err != nil {
		return res, err
	}
	podList := &corev1.PodList{}
	if err := r.client.List(context.TODO(), podList, &client.ListOptions{
		LabelSelector: client.MatchingLabelsSelector{
			Selector: selector,
		},
	}); err != nil {
		return res, err
	}
	nodeNames := make(map[string]struct{})
	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, ds) && pod.DeletionTimestamp == nil {
			nodeNames[pod.Spec.NodeName] = struct{}{}
		}
	}
	logger.Info("Found pods on the following nodes", "nodes", nodeNames)
	for nodeName := range nodeNames {
		node := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: nodeName,
			},
		}
		if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(node), node); err != nil {
			return res, err
		}
		res = append(res, *node)
	}
	return res, nil
}

func (r *ReconcileHostPathProvisioner) getStorageClassNameOrDefault(template *corev1.PersistentVolumeClaimSpec) string {
	if template != nil && template.StorageClassName != nil && len(*template.StorageClassName) > 0 {
		return *template.StorageClassName
	}
	return defaultStorageClassName
}

func (r *ReconcileHostPathProvisioner) storagePoolPVCByNode(storagePool *hostpathprovisionerv1.StoragePool, namespace string, node *corev1.Node) *corev1.PersistentVolumeClaim {
	labels := getRecommendedLabels()
	labels[storagePoolLabelKey] = r.getStorageClassNameOrDefault(storagePool.PVCTemplate)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      getStoragePoolPVCName(r.getStorageClassNameOrDefault(storagePool.PVCTemplate), node.GetName()),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: *storagePool.PVCTemplate,
	}
}

func (r *ReconcileHostPathProvisioner) storagePoolDeploymentByNode(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, sourceStoragePool *hostpathprovisionerv1.StoragePool, namespace string, node *corev1.Node) *appsv1.Deployment {
	args := getDaemonSetArgs(logger, namespace, false)
	labels := getRecommendedLabels()
	labels[storagePoolLabelKey] = sourceStoragePool.Name
	replicaCount := int32(1)
	directory := corev1.HostPathDirectory
	bidirectional := corev1.MountPropagationBidirectional
	privileged := true
	defaultGracePeriod := int64(30)
	progressDeadline := int32(600)
	revisionHistoryLimit := int32(10)

	dataMountPath := blockDataMountPath
	if sourceStoragePool.PVCTemplate.VolumeMode == nil || *sourceStoragePool.PVCTemplate.VolumeMode == corev1.PersistentVolumeFilesystem {
		dataMountPath = fsDataMountPath
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("hpp-pool-%s-%s", sourceStoragePool.Name, node.GetName()),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"hpp-pool": sourceStoragePool.Name,
				},
			},
			Replicas: &replicaCount,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						IntVal: int32(1),
					},
					MaxSurge: &intstr.IntOrString{
						IntVal: int32(2),
					},
				},
			},
			ProgressDeadlineSeconds: &progressDeadline,
			RevisionHistoryLimit:    &revisionHistoryLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:      sourceStoragePool.Name,
					Namespace: namespace,
					Labels: map[string]string{
						"hpp-pool": sourceStoragePool.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            ProvisionerServiceAccountNameCsi,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: &defaultGracePeriod,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values: []string{
													node.GetName(),
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "mounter",
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Image:           args.operatorImage,
							Command: []string{
								"/usr/bin/mounter",
								"--storagePoolPath",
								dataMountPath,
								"--mountPath",
								filepath.Join(sourceStoragePool.Path, "csi"),
								"--hostPath",
								"/host",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "host-root",
									MountPath:        "/host",
									MountPropagation: &bidirectional,
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							TerminationMessagePath:   "/dev/termination-log",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: dataName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: getStoragePoolPVCName(r.getStorageClassNameOrDefault(sourceStoragePool.PVCTemplate), node.GetName()),
								},
							},
						},
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: &directory,
								},
							},
						},
					},
				},
			},
		},
	}

	if sourceStoragePool.PVCTemplate.VolumeMode == nil || *sourceStoragePool.PVCTemplate.VolumeMode == corev1.PersistentVolumeFilesystem {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      dataName,
			MountPath: fsDataMountPath,
		})
	} else {
		deployment.Spec.Template.Spec.Containers[0].VolumeDevices = append(deployment.Spec.Template.Spec.Containers[0].VolumeDevices, corev1.VolumeDevice{
			Name:       dataName,
			DevicePath: blockDataMountPath,
		})
	}
	return deployment
}

func getStoragePoolPVCName(poolName, nodeName string) string {
	return fmt.Sprintf("hpp-pool-%s-%s", poolName, nodeName)
}

func (r *ReconcileHostPathProvisioner) storagePoolDeploymentsByStoragePool(cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, storagePool *hostpathprovisionerv1.StoragePool) ([]appsv1.Deployment, error) {
	res := make([]appsv1.Deployment, 0)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app":           MultiPurposeHostPathProvisionerName,
			storagePoolLabelKey: r.getStorageClassNameOrDefault(storagePool.PVCTemplate),
		},
	})
	if err != nil {
		return res, err
	}
	deploymentList := &appsv1.DeploymentList{}
	if err := r.client.List(context.TODO(), deploymentList, &client.ListOptions{
		LabelSelector: client.MatchingLabelsSelector{
			Selector: selector,
		},
	}); err != nil {
		return res, err
	}

	for _, deployment := range deploymentList.Items {
		if metav1.IsControlledBy(&deployment, cr) {
			res = append(res, deployment)
		}
	}

	return res, nil
}

func (r *ReconcileHostPathProvisioner) getClaimStatusesByStoragePool(storagePool *hostpathprovisionerv1.StoragePool, namespace string) ([]hostpathprovisionerv1.ClaimStatus, error) {
	res := make([]hostpathprovisionerv1.ClaimStatus, 0)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app":           MultiPurposeHostPathProvisionerName,
			storagePoolLabelKey: r.getStorageClassNameOrDefault(storagePool.PVCTemplate),
		},
	})
	if err != nil {
		return res, err
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.client.List(context.TODO(), pvcList, &client.ListOptions{
		LabelSelector: client.MatchingLabelsSelector{
			Selector: selector,
		},
	}); err != nil {
		return res, err
	}
	for _, pvc := range pvcList.Items {
		res = append(res, hostpathprovisionerv1.ClaimStatus{
			Name:   pvc.GetName(),
			Status: pvc.Status,
		})
	}

	return res, nil
}

func (r *ReconcileHostPathProvisioner) reconcileStoragePoolStatus(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) error {
	// Check the template of the storage pool
	if cr.Spec.PathConfig != nil {
		cr.Status.StoragePoolStatuses = append(cr.Status.StoragePoolStatuses, hostpathprovisionerv1.StoragePoolStatus{
			Name:  legacyStoragePoolName,
			Phase: hostpathprovisionerv1.StoragePoolReady,
		})
	} else {
		newStoragePoolStatuses := make([]hostpathprovisionerv1.StoragePoolStatus, 0)
		for _, storagePool := range cr.Spec.StoragePools {
			if storagePool.PVCTemplate != nil {
				deployments, err := r.storagePoolDeploymentsByStoragePool(cr, namespace, &storagePool)
				if err != nil {
					return err
				}
				logger.WithName("Status").Info("Number of deployments for pool", "storage pool", storagePool.Name, "deployment count", len(deployments))
				currentReady := 0
				for _, deployment := range deployments {
					if deployment.Status.ReadyReplicas == int32(1) {
						currentReady++
					}
				}
				logger.WithName("Status").Info("Number of deployments for pool ready", "storage pool", storagePool.Name, "deployment count", currentReady)
				claimStatuses, err := r.getClaimStatusesByStoragePool(&storagePool, namespace)
				if err != nil {
					return err
				}

				newStoragePoolStatuses = append(newStoragePoolStatuses, hostpathprovisionerv1.StoragePoolStatus{
					Name:          storagePool.Name,
					Phase:         hostpathprovisionerv1.StoragePoolReady,
					DesiredReady:  len(deployments),
					CurrentReady:  currentReady,
					ClaimStatuses: claimStatuses,
				})
			} else {
				newStoragePoolStatuses = append(newStoragePoolStatuses, hostpathprovisionerv1.StoragePoolStatus{
					Name:  storagePool.Name,
					Phase: hostpathprovisionerv1.StoragePoolReady,
				})
			}
		}
		cr.Status.StoragePoolStatuses = newStoragePoolStatuses
	}
	return nil
}

func (r *ReconcileHostPathProvisioner) hasCleanUpFinished() (bool, error) {
	jobList := &batchv1.JobList{}
	if err := r.client.List(context.TODO(), jobList); err != nil {
		return false, err
	}
	return len(jobList.Items) > 0, nil
}

func (r *ReconcileHostPathProvisioner) createCleanupJobForNode(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string, sourceStoragePool *hostpathprovisionerv1.StoragePool, node *corev1.Node) error {
	args := getDaemonSetArgs(logger, namespace, false)
	labels := getRecommendedLabels()
	ttl := int32(10)
	defaultGracePeriod := int64(30)
	privileged := true
	directory := corev1.HostPathDirectory
	bidirectional := corev1.MountPropagationBidirectional
	cleanupJob := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("cleanup-pool-%s-%s", sourceStoragePool.Name, node.GetName()),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName:            ProvisionerServiceAccountNameCsi,
					RestartPolicy:                 corev1.RestartPolicyOnFailure,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: &defaultGracePeriod,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values: []string{
													node.GetName(),
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "mounter",
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Image:           args.operatorImage,
							Command: []string{
								"/usr/bin/mounter",
								"--mountPath",
								filepath.Join(sourceStoragePool.Path, "csi"),
								"--hostPath",
								"/host",
								"--unmount",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "host-root",
									MountPath:        "/host",
									MountPropagation: &bidirectional,
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							TerminationMessagePath:   "/dev/termination-log",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: &directory,
								},
							},
						},
					},
				},
			},
		},
	}
	logger.Info("Creating cleanup job", "name", cleanupJob.Name)
	if err := r.client.Create(context.TODO(), cleanupJob); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error(err, "Unable to create cleanup job", "name", cleanupJob.GetName())
	}
	return nil
}
