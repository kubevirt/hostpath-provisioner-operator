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

GOLANG_VER=${GOLANG_VER:-1.19.12}

function ensureArmAvailable() {
  if [[ -v PROW_JOB_ID && GOARCH="arm64" ]] ; then
    dnf install -y qemu-user-static
  fi
}
