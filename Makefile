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
GOOS?=linux
GOARCH?=amd64
BUILDAH_TLS_VERIFY?=true
BUILDAH_PLATFORM_FLAG?=--platform $(GOOS)/$(GOARCH)
GOLANG_VER?=1.20.8

export GOLANG_VER
export TAG
export GOOS
export GOARCH

all: test build

operator:
	./hack/build-operator.sh

mounter:
	./hack/build-mounter.sh

csv-generator:
	./hack/build-csv-generator.sh

crd-generator: generate-crd
	./hack/build-crd-generator.sh
	_out/crd-generator --sourcefile=./deploy/operator.yaml --outputDir=./tools/helper

image: operator mounter csv-generator
	./hack/version.sh ./_out; \
	buildah build $(BUILDAH_PLATFORM_FLAG) -t $(DOCKER_REPO)/$(OPERATOR_IMAGE):$(GOARCH) -f Dockerfile .

manifest: image
	-buildah manifest create $(DOCKER_REPO)/$(OPERATOR_IMAGE):local
	buildah manifest add --arch $(GOARCH) $(DOCKER_REPO)/$(OPERATOR_IMAGE):local containers-storage:$(DOCKER_REPO)/$(OPERATOR_IMAGE):$(GOARCH)

push: clean manifest manifest-push

manifest-push:
	buildah manifest push --tls-verify=${BUILDAH_TLS_VERIFY} --all $(DOCKER_REPO)/$(OPERATOR_IMAGE):local docker://$(DOCKER_REPO)/$(OPERATOR_IMAGE):$(TAG)

generate:
	./hack/update-codegen.sh

generate-crd:
	./hack/generate-crd.sh

manifest-clean:
	-buildah manifest rm $(DOCKER_REPO)/$(OPERATOR_IMAGE):local

clean: manifest-clean
	GO111MODULE=on; \
	go mod tidy; \
	go mod vendor; \
	rm -rf _out

build: clean operator crd-generator csv-generator

test:
	hack/run-lint-checks.sh
	hack/language.sh
	go test -v ./cmd/... ./pkg/... ./tools/... ./version/...

lint-metrics:
	./hack/prom_metric_linter.sh --operator-name="kubevirt" --sub-operator-name="hpp"

generate-doc: build-docgen
	_out/metricsdocs > docs/metrics.md

build-docgen:
	go build -ldflags="-s -w" -o _out/metricsdocs ./tools/metricsdocs
