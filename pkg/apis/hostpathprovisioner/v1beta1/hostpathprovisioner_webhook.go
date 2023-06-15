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

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	maxStoragePoolNameLength   = 50
	maxPathLength              = 255
	pathConfigAndPoolSet       = "pathConfig and storage pools cannot be both set"
	pathConfigOrPoolNotSet     = "either pathConfig or storage pools must be set"
	pathConfigMustBeSet        = "pathconfig path must be set"
	poolNameCannotBeBlank      = "storagePool.name cannot be blank"
	poolNameCannotBeGreater50  = "storagePool.name cannot have a length greater than 50"
	poolPathCannotBeBlank      = "storagePool.path cannot be blank"
	poolPathCannotBeGreater255 = "storagePool.path cannot have a length greater than 255"
)

// SetupWebhookWithManager configures the webhook for the passed in manager
func (r *HostPathProvisioner) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Validator = &HostPathProvisioner{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HostPathProvisioner) ValidateCreate() (admission.Warnings, error) {
	return r.validatePathConfigAndStoragePools()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HostPathProvisioner) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	return r.validatePathConfigAndStoragePools()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HostPathProvisioner) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *HostPathProvisioner) validatePathConfigAndStoragePools() (admission.Warnings, error) {
	if r.Spec.PathConfig != nil && len(r.Spec.StoragePools) > 0 {
		return admission.Warnings{pathConfigAndPoolSet}, fmt.Errorf(pathConfigAndPoolSet)
	} else if r.Spec.PathConfig == nil && len(r.Spec.StoragePools) == 0 {
		return admission.Warnings{pathConfigOrPoolNotSet}, fmt.Errorf(pathConfigOrPoolNotSet)
	}
	if r.Spec.PathConfig != nil && len(r.Spec.PathConfig.Path) == 0 {
		return admission.Warnings{pathConfigMustBeSet}, fmt.Errorf(pathConfigMustBeSet)
	}
	usedPaths := make(map[string]int, 0)
	usedNames := make(map[string]int, 0)
	for i, source := range r.Spec.StoragePools {
		if warning, err := validateStoragePool(source); err != nil {
			return warning, err
		}
		if index, ok := usedPaths[source.Path]; !ok {
			usedPaths[source.Path] = i
		} else {
			errorString := fmt.Sprintf("spec.storagePools[%d].path is the same as spec.storagePools[%d].path, cannot have duplicate paths", i, index)
			return admission.Warnings{errorString}, fmt.Errorf("%s", errorString)
		}
		if index, ok := usedNames[source.Name]; !ok {
			usedNames[source.Name] = i
		} else {
			errorString := fmt.Sprintf("spec.storagePools[%d].name is the same as spec.storagePools[%d].name, cannot have duplicate names", i, index)
			return admission.Warnings{errorString}, fmt.Errorf("%s", errorString)
		}
	}
	return nil, nil
}

func validateStoragePool(storagePool StoragePool) (admission.Warnings, error) {
	if storagePool.Name == "" {
		return admission.Warnings{poolNameCannotBeBlank}, fmt.Errorf(poolNameCannotBeBlank)
	}
	if len(storagePool.Name) > maxStoragePoolNameLength {
		return admission.Warnings{poolNameCannotBeGreater50}, fmt.Errorf(poolNameCannotBeGreater50)
	}
	if storagePool.Path == "" {
		return admission.Warnings{poolPathCannotBeBlank}, fmt.Errorf(poolPathCannotBeBlank)
	}
	if len(storagePool.Path) > maxPathLength {
		return admission.Warnings{poolPathCannotBeGreater255}, fmt.Errorf(poolPathCannotBeGreater255)
	}
	return nil, nil
}
