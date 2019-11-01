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

const (
	// OperatorImageDefault is the default value of the operator container image name.
	OperatorImageDefault = "hostpath-provisioner-operator"
	// ProvisionerImageDefault is the default value of the provisioner container image name.
	ProvisionerImageDefault    = "hostpath-provisioner"
	provisionerImageEnvVarName = "PROVISIONER_IMAGE"
	// OperatorServiceAccountName is the name of Service Account used to run the operator.
	OperatorServiceAccountName = "hostpath-provisioner-operator"
)
