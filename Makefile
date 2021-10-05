# Copyright 2019 The hostpath provisioner operator Authors.
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

OPERATOR_IMAGE?=hostpath-provisioner-operator
TAG?=latest
DOCKER_REPO?=quay.io/kubevirt

all: test build

operator:
	GOLANG_VER=${GOLANG_VER} ./hack/build-operator.sh

csv-generator:
	GOLANG_VER=${GOLANG_VER} ./hack/build-csv-generator.sh

image: operator csv-generator
	hack/version.sh _out; \
	docker build -t $(DOCKER_REPO)/$(OPERATOR_IMAGE):$(TAG) -f Dockerfile .

push: image
	docker push $(DOCKER_REPO)/$(OPERATOR_IMAGE):$(TAG)

generate:
	./hack/update-codegen.sh

generate-crd:
	controller-gen crd:crdVersions=v1 output:dir=./deploy/ paths=./pkg/apis/hostpathprovisioner/...

clean:
	GO111MODULE=on; \
	go mod tidy; \
	go mod vendor; \
	rm -rf _out

build: clean operator csv-generator

test:
	hack/run-lint-checks.sh
	hack/language.sh
	go test -v ./pkg/... ./tools/... ./version/...
