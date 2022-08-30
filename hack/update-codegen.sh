#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail
set -x

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

find "${SCRIPT_ROOT}/pkg/" -name "*generated*.go" -exec rm {} -f \;
rm -rf "${SCRIPT_ROOT}/pkg/client"

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  kubevirt.io/hostpath-provisioner-operator/pkg/client \
  kubevirt.io/hostpath-provisioner-operator/pkg/apis \
  "hostpathprovisioner:v1beta1" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  --output-base $GOPATH


go install ${CODEGEN_PKG}/cmd/openapi-gen
openapi-gen --logtostderr=true -o "" -i kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1 -O zz_generated.openapi -p ./pkg/apis/hostpathprovisioner/v1beta1 -h ./hack/boilerplate.go.txt -r "-"

