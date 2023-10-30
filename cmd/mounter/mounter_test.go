/*
Copyright 2021 The hostpath provisioner operator Authors.

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
package main

import (
	//revive:disable
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	//revive:enable
)

var _ = Describe("Mounter tests", func() {
	Context("lsblk JSON parsing", func() {
		intJSON := `{
			"blockdevices": [
			   {"name": "vdb", "maj:min": "252:16", "rm": "1", "size": "120G", "ro": "0", "type": "disk", "mountpoint": "/host/var/hpvolumes/csi"}
			]
		 }`
		boolJSON := `{
			"blockdevices": [
			   {"name":"rbd0", "maj:min":"251:0", "rm":true, "size":"120G", "ro":false, "type":"disk", "mountpoint":"/host/var/hpvolumes/csi"}
			]
		 }`

		table.DescribeTable("should not panic over json booleans with", func(jsonStr string) {
			// Override lsblk cmd
			lsblkCommand = func(source string) ([]byte, error) {
				Expect(source).To(Equal("test"))
				return []byte(jsonStr), nil
			}

			deviceInfos, err := lookupDeviceInfoByVolume("test")
			Expect(err).ToNot(HaveOccurred())
			Expect(deviceInfos[0].Size).To(Equal("120G"))
		},
			table.Entry("integer value", intJSON),
			table.Entry("actual boolean", boolJSON),
		)
	})
})
