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

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
SCRIPT_ROOT="${SCRIPT_DIR}/.."
CODEGEN_PKG="${CODEGEN_PKG:-"${SCRIPT_ROOT}/vendor/k8s.io/code-generator"}"

echo "SCRIPT_DIR $SCRIPT_DIR, SCRIPT_ROOT $SCRIPT_ROOT, CODEGEN_PKG $CODEGEN_PKG"

find "${SCRIPT_ROOT}/pkg/" -name "*generated*.go" -exec rm {} -f \;
rm -rf "${SCRIPT_ROOT}/pkg/client"

source "$CODEGEN_PKG/kube_codegen.sh"

report_filename="${SCRIPT_DIR}/codegen_violation_exceptions.list"
update_report="--update-report"

kube::codegen::gen_helpers \
	--input-pkg-root kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1 \
	--output-base "${SCRIPT_ROOT}/../.." \
        --boilerplate "${SCRIPT_DIR}"/boilerplate.go.txt

kube::codegen::gen_openapi \
    --input-pkg-root kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1 \
    --output-pkg-root kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1 \
    --output-base "${SCRIPT_ROOT}/../.." \
    --openapi-name "" \
    --report-filename "${report_filename:-"/dev/null"}" \
    --update-report \
    --boilerplate "${SCRIPT_DIR}/boilerplate.go.txt"

kube::codegen::gen_client \
    --with-watch \
    --with-applyconfig \
    --input-pkg-root kubevirt.io/hostpath-provisioner-operator/pkg/apis \
    --output-pkg-root kubevirt.io/hostpath-provisioner-operator/pkg/client \
    --output-base "${SCRIPT_ROOT}/../.." \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

