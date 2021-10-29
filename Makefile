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

csv-generator: crd-generator
	GOLANG_VER=${GOLANG_VER} ./hack/build-csv-generator.sh

crd-generator: generate-crd
	GOLANG_VER=${GOLANG_VER} ./hack/build-crd-generator.sh
	_out/crd-generator --sourcefile=./deploy/operator.yaml --outputDir=./tools/helper

image: operator csv-generator
	TAG=$(TAG) ./hack/version.sh ./_out; \
	docker build -t $(DOCKER_REPO)/$(OPERATOR_IMAGE):$(TAG) -f Dockerfile .

push: image
	docker push $(DOCKER_REPO)/$(OPERATOR_IMAGE):$(TAG)

generate:
	./hack/update-codegen.sh

generate-crd:
	./hack/generate-crd.sh

clean:
	GO111MODULE=on; \
	go mod tidy; \
	go mod vendor; \
	rm -rf _out

build: clean operator crd-generator csv-generator

test:
	hack/run-lint-checks.sh
	hack/language.sh
	go test -v ./pkg/... ./tools/... ./version/...
