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
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Mounter tests", func() {
	ginkgo.Context("filterPodMounts", func() {
		ginkgo.It("should filter to pod mount when multiple mounts exist", func() {
			infos := []FindmntInfo{
				{
					Target: "/var/lib/kubelet/plugins/kubernetes.io/csi/openshift-storage.cephfs.csi.ceph.com/abc123/globalmount",
					Source: "csi-cephfs-node@cluster.cephfs=/volumes/csi/csi-vol-123/abc",
				},
				{
					Target: "/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-abc/mount",
					Source: "csi-cephfs-node@cluster.cephfs=/volumes/csi/csi-vol-123/abc",
				},
			}
			result := filterPodMounts(infos)
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Target).To(gomega.ContainSubstring("/pods/"))
		})

		ginkgo.It("should return all infos when no pod mount is found", func() {
			infos := []FindmntInfo{
				{
					Target: "/var/lib/kubelet/plugins/kubernetes.io/csi/driver/globalmount",
					Source: "some-source",
				},
				{
					Target: "/var/lib/kubelet/plugins/kubernetes.io/csi/driver/other",
					Source: "some-source",
				},
			}
			result := filterPodMounts(infos)
			gomega.Expect(result).To(gomega.HaveLen(2))
		})

		ginkgo.It("should return single pod mount unchanged", func() {
			infos := []FindmntInfo{
				{
					Target: "/var/lib/kubelet/pods/pod-uid/volumes/kubernetes.io~csi/pvc-123/mount",
					Source: "nfs-server:/share",
				},
			}
			result := filterPodMounts(infos)
			gomega.Expect(result).To(gomega.HaveLen(1))
		})
	})

	ginkgo.Context("lsblk JSON parsing", func() {
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

		ginkgo.DescribeTable("should not panic over json booleans with", func(jsonStr string) {
			// Override lsblk cmd
			lsblkCommand = func(source string) ([]byte, error) {
				gomega.Expect(source).To(gomega.Equal("test"))
				return []byte(jsonStr), nil
			}

			deviceInfos, err := lookupDeviceInfoByVolume("test")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(deviceInfos[0].Size).To(gomega.Equal("120G"))
		},
			ginkgo.Entry("integer value", intJSON),
			ginkgo.Entry("actual boolean", boolJSON),
		)
	})
})
