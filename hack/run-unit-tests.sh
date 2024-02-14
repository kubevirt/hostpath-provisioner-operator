#!/usr/bin/env bash

#Copyright 2021 The hostpath provisioner operator Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
set -e

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
go version
# Validate
make lint-metrics
make generate-doc
git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

# Install dependencies for the test run
if [[ -v PROW_JOB_ID ]] ; then
  cd /home/prow/go/src/github.com/kubevirt/hostpath-provisioner-operator
  go get -u github.com/mgechev/revive
  go mod vendor
fi
# Run test
make test
