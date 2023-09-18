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
package version

import (
	"os"
	"path/filepath"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Version", func() {
	var orgFunc func() (string, error)

	ginkgo.BeforeEach(func() {
		orgFunc = VersionStringFunc
	})

	ginkgo.AfterEach(func() {
		VersionStringFunc = orgFunc
	})

	ginkgo.It("should return error on invalid string", func() {
		VersionStringFunc = func() (string, error) {
			return "latest", nil
		}
		_, err := GetVersion()
		gomega.Expect(err).To(gomega.HaveOccurred())
	})

	ginkgo.It("should return 0.0.1 on v0.0.1", func() {
		VersionStringFunc = func() (string, error) {
			return "v0.0.1", nil
		}
		result, err := GetVersion()
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(result.String()).To(gomega.Equal("0.0.1"))
	})

	ginkgo.It("should return 1.0.1 on 1.0.1", func() {
		VersionStringFunc = func() (string, error) {
			return "1.0.1", nil
		}
		result, err := GetVersion()
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(result.String()).To(gomega.Equal("1.0.1"))
	})
})

var _ = ginkgo.Describe("GetStringFromFile", func() {
	ginkgo.It("should return nil on invalid file", func() {
		result, err := GetStringFromFile("invalid")
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(result).To(gomega.Equal(""))
	})

	ginkgo.It("Should return valid string", func() {
		tmpDir, err := os.MkdirTemp("", "version")
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		testFile := filepath.Join(tmpDir, "version.txt")
		os.WriteFile(testFile, []byte("v1.1.1"), 0644)
		result, err := GetStringFromFile(testFile)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(result).To(gomega.Equal("v1.1.1"))
	})
})
