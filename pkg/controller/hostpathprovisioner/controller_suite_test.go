/*
Copyright 2019 The hostpath provisioner operator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package hostpathprovisioner

import (
	"os"
	"testing"

	"github.com/go-logr/zapr"
	//revive:disable
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//revive:enable
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zapr.NewLogger(zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(GinkgoWriter), zap.DebugLevel))))
	os.Setenv(PartOfLabelEnvVarName, "testing")
	os.Setenv(VersionLabelEnvVarName, "v0.0.0-tests")
})

var _ = AfterSuite(func() {
	os.Unsetenv(PartOfLabelEnvVarName)
	os.Unsetenv(VersionLabelEnvVarName)
})

func TestHostpathProvisioners(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hostpath Provisioner Suite")
}
