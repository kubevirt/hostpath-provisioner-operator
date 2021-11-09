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
	"github.com/go-logr/logr"
	hostpathprovisionerv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func (r *ReconcileHostPathProvisioner) reconcileStoragePool(logger logr.Logger, cr *hostpathprovisionerv1.HostPathProvisioner, namespace string) error {
	// Check the template of the storage pool
	if cr.Spec.PathConfig != nil {
		cr.Status.StoragePoolStatuses = append(cr.Status.StoragePoolStatuses, hostpathprovisionerv1.StoragePoolStatus{
			Name:  legacyStoragePoolName,
			Phase: hostpathprovisionerv1.StoragePoolReady,
		})
	} else {
		for _, storagePool := range cr.Spec.StoragePools {
			if storagePool.StorageClass != nil {

			} else {
				cr.Status.StoragePoolStatuses = append(cr.Status.StoragePoolStatuses, hostpathprovisionerv1.StoragePoolStatus{
					Name:  storagePool.Name,
					Phase: hostpathprovisionerv1.StoragePoolReady,
				})
			}
		}
	}
	return nil
}
