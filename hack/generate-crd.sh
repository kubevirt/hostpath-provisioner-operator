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

function ensureControllerGen() {
  if ! type controller-gen ; then
    go get sigs.k8s.io/controller-tools/cmd/controller-gen
  fi
}

ensureControllerGen
controller-gen crd:crdVersions=v1 output:dir=./deploy/ paths=./pkg/apis/hostpathprovisioner/...

# First remove the CRD from operator.yaml and replace it with a marker #######
sed -z 's/---\napiVersion: apiextensions\.k8s\.io\/v1\nkind: CustomResourceDefinition.*---/######\n---/' ./deploy/operator.yaml > ./deploy/operator.tmp.yaml
# Second take the generated file from the command above and insert it right after the marker, the /r<filename> is what does the insert
sed -ie '/######/rdeploy/hostpathprovisioner.kubevirt.io_hostpathprovisioners.yaml' ./deploy/operator.tmp.yaml
# Third remove the marker from the file. The generated file has some newlines at the top that should also get removed.
sed -z 's/######\n\n//' ./deploy/operator.tmp.yaml > ./deploy/operator.yaml
# Clean up any temporary files left over.
rm deploy/operator.tmp.yaml*
rm deploy/hostpathprovisioner.kubevirt.io_hostpathprovisioners.yaml
