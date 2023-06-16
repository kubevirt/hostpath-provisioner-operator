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
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	var orgFunc func() (string, error)

	BeforeEach(func() {
		orgFunc = VersionStringFunc
	})

	AfterEach(func() {
		VersionStringFunc = orgFunc
	})

	It("should return error on invalid string", func() {
		VersionStringFunc = func() (string, error) {
			return "latest", nil
		}
		_, err := GetVersion()
		Expect(err).To(HaveOccurred())
	})

	It("should return 0.0.1 on v0.0.1", func() {
		VersionStringFunc = func() (string, error) {
			return "v0.0.1", nil
		}
		result, err := GetVersion()
		Expect(err).ToNot(HaveOccurred())
		Expect(result.String()).To(Equal("0.0.1"))
	})

	It("should return 1.0.1 on 1.0.1", func() {
		VersionStringFunc = func() (string, error) {
			return "1.0.1", nil
		}
		result, err := GetVersion()
		Expect(err).ToNot(HaveOccurred())
		Expect(result.String()).To(Equal("1.0.1"))
	})
})

var _ = Describe("GetStringFromFile", func() {
	It("should return nil on invalid file", func() {
		result, err := GetStringFromFile("invalid")
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(""))
	})

	It("Should return valid string", func() {
		tmpDir, err := ioutil.TempDir("", "version")
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(tmpDir, "version.txt")
		ioutil.WriteFile(testFile, []byte("v1.1.1"), 0644)
		result, err := GetStringFromFile(testFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("v1.1.1"))
	})
})
