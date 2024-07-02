module kubevirt.io/hostpath-provisioner-operator

require (
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/blang/semver v3.5.1+incompatible
	github.com/blang/semver/v4 v4.0.0
	github.com/evanphx/json-patch v5.9.0+incompatible
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v1.4.2
	github.com/go-logr/zapr v1.3.0
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20230710120526-cc1644c90b64
	github.com/machadovilaca/operator-observability v0.0.20
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/custom-resource-status v1.1.2
	github.com/operator-framework/api v0.23.0
	github.com/operator-framework/operator-sdk v0.16.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.75.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.27.0
	k8s.io/api v0.30.2
	k8s.io/apiextensions-apiserver v0.30.2
	k8s.io/apimachinery v0.30.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.28.1
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340
	k8s.io/utils v0.0.0-20240502163921-fe8a2dddb1d0
	sigs.k8s.io/controller-runtime v0.18.4
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240424215950-a892ee059fd6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.47.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sirupsen/logrus v1.9.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20240213143201-ec583247a57a // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/term v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/gengo v0.0.0-20230829151522-9cce18d56c01 // indirect
	k8s.io/gengo/v2 v2.0.0-20240228010128-51d4e06bde70 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm
	github.com/openshift/api => github.com/openshift/api v0.0.0-20211217221424-8779abfbd571
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190424153033-d3245f150225
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
	k8s.io/api => k8s.io/api v0.28.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.28.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.28.1
	k8s.io/apiserver => k8s.io/apiserver v0.28.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.28.1
	k8s.io/client-go => k8s.io/client-go v0.28.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.28.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.28.1
	k8s.io/code-generator => k8s.io/code-generator v0.28.1
	k8s.io/component-base => k8s.io/component-base v0.28.1
	k8s.io/cri-api => k8s.io/cri-api v0.28.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.28.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.28.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.28.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.28.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.28.1
	k8s.io/kubectl => k8s.io/kubectl v0.28.1
	k8s.io/kubelet => k8s.io/kubelet v0.28.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.28.1
	k8s.io/metrics => k8s.io/metrics v0.28.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.28.1
)

go 1.21
toolchain go1.22.4
