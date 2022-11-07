#!/bin/bash

#  This file is part of the hostpath-provisioner-operator project
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#
#  Copyright 2022 Red Hat, Inc.
#

set -ex

source ./cluster/config.sh
source ./cluster/kubevirtci.sh

KUBEVIRTCI_PATH=$(kubevirtci::path)
KUBECTL=$(which kubectl 2> /dev/null) || (echo "could not find kubectl executable" && exit 1)

export KUBECONFIG=$(kubevirtci::kubeconfig)

# install cert-manager
${KUBECTL} apply -f https://github.com/cert-manager/cert-manager/releases/download/v"${CERT_MANAGER_VERSION}"/cert-manager.yaml
${KUBECTL} wait --for=condition=Available --namespace cert-manager --timeout 120s --all deployments

# install hostpath-provisioner-operator
make build

${KUBECTL} apply -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/namespace.yaml
${KUBECTL} apply -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/webhook.yaml --namespace hostpath-provisioner
${KUBECTL} apply -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/operator.yaml --namespace hostpath-provisioner
