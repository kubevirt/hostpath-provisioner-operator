#!/usr/bin/env bash

#Copyright 2019 The hostpath provisioner operator Authors.
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

# NOTE: Not using pipefail because gofmt returns 0 when it finds
# suggestions and 1 when files are clean

#install revive if it is not there yet
go install github.com/mgechev/revive@latest

SOURCE_DIRS="pkg cmd version"
LINTABLE=(cmd pkg version)
ec=0
out="$(gofmt -l -s ${SOURCE_DIRS} | grep ".*\.go")"
if [[ ${out} ]]; then
    echo "FAIL: Format errors found in the following files:"
    echo "${out}"
    ec=1
fi
for p in "${LINTABLE[@]}"; do
  echo "running revive on directory: ${p}"
  out="$($GOPATH/bin/revive -formatter friendly -exclude pkg/apis/hostpathprovisioner/v1beta1/zz_generated.openapi.go ${p}/...)"
  if [[ ${out} ]]; then
    echo "FAIL: following revive errors found:"
    echo "${out}"
    ec=1
  fi
done

exit ${ec}
