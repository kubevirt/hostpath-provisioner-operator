module kubevirt.io/hostpath-provisioner-operator

require (
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/blang/semver v3.5.1+incompatible
	github.com/blang/semver/v4 v4.0.0
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v1.2.4
	github.com/go-logr/zapr v1.2.4
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20230710120526-cc1644c90b64
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.27.8
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/custom-resource-status v1.1.2
	github.com/operator-framework/api v0.17.6
	github.com/operator-framework/operator-sdk v0.16.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/client_model v0.4.0
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.24.0
	k8s.io/api v0.26.1
	k8s.io/apiextensions-apiserver v0.26.1
	k8s.io/apimachinery v0.26.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.26.1
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280
	k8s.io/utils v0.0.0-20221128185143-99ec85e7a448
	sigs.k8s.io/controller-runtime v0.14.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.5.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/component-base v0.26.1 // indirect
	k8s.io/gengo v0.0.0-20220902162205-c0856e24416d // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.26.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.1
	k8s.io/apiserver => k8s.io/apiserver v0.26.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.1
	k8s.io/client-go => k8s.io/client-go v0.26.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.1
	k8s.io/code-generator => k8s.io/code-generator v0.26.1
	k8s.io/component-base => k8s.io/component-base v0.26.1
	k8s.io/cri-api => k8s.io/cri-api v0.26.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.1
	k8s.io/kubectl => k8s.io/kubectl v0.26.1
	k8s.io/kubelet => k8s.io/kubelet v0.26.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.1
	k8s.io/metrics => k8s.io/metrics v0.26.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.1
)

replace (
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm
	github.com/openshift/api => github.com/openshift/api v0.0.0-20211217221424-8779abfbd571
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190424153033-d3245f150225
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
)

go 1.19
