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
	${SCRIPT_ROOT}/pkg/apis \
        --boilerplate "${SCRIPT_DIR}"/boilerplate.go.txt

kube::codegen::gen_openapi \
	${SCRIPT_ROOT}/pkg/apis \
	--output-pkg kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1 \
	--output-dir "${SCRIPT_ROOT}"/pkg/apis/hostpathprovisioner/v1beta1 \
	--report-filename "${report_filename:-"/dev/null"}" \
	--update-report \
	--boilerplate "${SCRIPT_DIR}/boilerplate.go.txt"

kube::codegen::gen_client \
	${SCRIPT_ROOT}/pkg/apis \
	--with-watch \
	--with-applyconfig \
	--output-pkg kubevirt.io/hostpath-provisioner-operator/pkg/client \
	--output-dir "${SCRIPT_ROOT}/pkg/client" \
	--boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

