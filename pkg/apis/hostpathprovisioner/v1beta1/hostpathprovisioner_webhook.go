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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	maxStoragePoolNameLength = 50
	maxPathLength            = 255
)

// SetupWebhookWithManager configures the webhook for the passed in manager
func (r *HostPathProvisioner) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(&HostPathProvisionerValidator{}).
		Complete()
}

type HostPathProvisionerValidator struct {
}

var _ admission.Validator[*HostPathProvisioner] = &HostPathProvisionerValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *HostPathProvisionerValidator) ValidateCreate(ctx context.Context, obj *HostPathProvisioner) (warnings admission.Warnings, err error) {
	return v.validateSpec(obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *HostPathProvisionerValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *HostPathProvisioner) (warnings admission.Warnings, err error) {
	return v.validateSpec(newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *HostPathProvisionerValidator) ValidateDelete(ctx context.Context, obj *HostPathProvisioner) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (v *HostPathProvisionerValidator) validateSpec(hpp *HostPathProvisioner) (admission.Warnings, error) {
	warnings, err := v.validatePathConfigAndStoragePools(hpp)
	if err != nil {
		return warnings, err
	}
	if err := validateComponentResources(hpp.Spec.Resources); err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (v *HostPathProvisionerValidator) validatePathConfigAndStoragePools(hpp *HostPathProvisioner) (admission.Warnings, error) {
	if hpp.Spec.PathConfig != nil && len(hpp.Spec.StoragePools) > 0 {
		return nil, fmt.Errorf("pathConfig and storage pools cannot be both set")
	} else if hpp.Spec.PathConfig == nil && len(hpp.Spec.StoragePools) == 0 {
		return nil, fmt.Errorf("either pathConfig or storage pools must be set")
	}
	if hpp.Spec.PathConfig != nil && len(hpp.Spec.PathConfig.Path) == 0 {
		return nil, fmt.Errorf("pathconfig path must be set")
	}
	usedPaths := make(map[string]int, 0)
	usedNames := make(map[string]int, 0)
	for i, source := range hpp.Spec.StoragePools {
		if err := validateStoragePool(source); err != nil {
			return nil, err
		}
		if index, ok := usedPaths[source.Path]; !ok {
			usedPaths[source.Path] = i
		} else {
			return nil, fmt.Errorf("spec.storagePools[%d].path is the same as spec.storagePools[%d].path, cannot have duplicate paths", i, index)
		}
		if index, ok := usedNames[source.Name]; !ok {
			usedNames[source.Name] = i
		} else {
			return nil, fmt.Errorf("spec.storagePools[%d].name is the same as spec.storagePools[%d].name, cannot have duplicate names", i, index)
		}
	}
	return nil, nil
}

func validateStoragePool(storagePool StoragePool) error {
	if storagePool.Name == "" {
		return fmt.Errorf("storagePool.name cannot be blank")
	}
	if len(storagePool.Name) > maxStoragePoolNameLength {
		return fmt.Errorf("storagePool.name cannot have a length greater than 50")
	}
	if storagePool.Path == "" {
		return fmt.Errorf("storagePool.path cannot be blank")
	}
	if len(storagePool.Path) > maxPathLength {
		return fmt.Errorf("storagePool.path cannot have a length greater than 255")
	}
	return nil
}

func validateComponentResources(resources *ComponentResources) error {
	if resources == nil {
		return nil
	}
	components := []struct {
		name         string
		requirements *corev1.ResourceRequirements
	}{
		{"hostPathProvisioner", resources.HostPathProvisioner},
		{"csiProvisioner", resources.CSIProvisioner},
		{"nodeDriverRegistrar", resources.NodeDriverRegistrar},
		{"livenessProbe", resources.LivenessProbe},
		{"csiSnapshotter", resources.CSISnapshotter},
		{"mounter", resources.Mounter},
	}
	for _, component := range components {
		if err := validateResourceRequirements(fmt.Sprintf("spec.resources.%s", component.name), component.requirements); err != nil {
			return err
		}
	}
	return nil
}

func validateResourceRequirements(fieldPath string, requirements *corev1.ResourceRequirements) error {
	if requirements == nil {
		return nil
	}
	for name, quantity := range requirements.Limits {
		if quantity.Sign() < 0 {
			return fmt.Errorf("%s.limits.%s cannot be negative", fieldPath, name)
		}
	}
	for name, quantity := range requirements.Requests {
		if quantity.Sign() < 0 {
			return fmt.Errorf("%s.requests.%s cannot be negative", fieldPath, name)
		}
		if limit, ok := requirements.Limits[name]; ok && quantity.Cmp(limit) > 0 {
			return fmt.Errorf("%s.requests.%s must be less than or equal to %s.limits.%s", fieldPath, name, fieldPath, name)
		}
	}
	return nil
}
