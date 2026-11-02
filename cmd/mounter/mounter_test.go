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
	"os"
	"syscall"

	mount "github.com/moby/sys/mountinfo"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Mounter tests", func() {
	ginkgo.Context("filterGlobalMounts", func() {
		ginkgo.It("should filter to global mount when multiple mounts exist", func() {
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
			result := filterGlobalMounts(infos)
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Target).To(gomega.ContainSubstring("/globalmount"))
		})

		ginkgo.It("should return empty when no global mount is found", func() {
			infos := []FindmntInfo{
				{
					Target: "/var/lib/kubelet/pods/pod-uid-1/volumes/kubernetes.io~csi/pvc-1/mount",
					Source: "some-source",
				},
				{
					Target: "/var/lib/kubelet/pods/pod-uid-2/volumes/kubernetes.io~csi/pvc-2/mount",
					Source: "some-source",
				},
			}
			result := filterGlobalMounts(infos)
			gomega.Expect(result).To(gomega.BeEmpty())
		})

		ginkgo.It("should return single global mount unchanged", func() {
			infos := []FindmntInfo{
				{
					Target: "/var/lib/kubelet/plugins/kubernetes.io/csi/driver/abc123/globalmount",
					Source: "nfs-server:/share",
				},
			}
			result := filterGlobalMounts(infos)
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

	ginkgo.Context("selectBindSource", func() {
		const (
			pool     = "/var/lib/kubelet/pods/mounter/volumes/kubernetes.io~csi/pool/mount"
			consumer = "/var/lib/kubelet/pods/consumer/volumes/kubernetes.io~csi/pvc/mount"
			target   = "/var/hpvolumes/csi"
		)

		// Every entry models an NFS mount with the pool's device number and
		// Root="/", so only directory identity distinguishes the pool.
		ginkgo.DescribeTable("should select the source directory", func(mountpoints []string, targetSource, stale, want string, wantErr bool) {
			poolInfo, err := os.Stat(ginkgo.GinkgoT().TempDir())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			consumerInfo, err := os.Stat(ginkgo.GinkgoT().TempDir())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			files := map[string]os.FileInfo{pool: poolInfo, consumer: consumerInfo}
			files[target] = files[targetSource]
			stat := func(path string) (os.FileInfo, error) {
				if info := files[path]; info != nil && path != stale {
					return info, nil
				}
				return nil, &os.PathError{Op: "stat", Path: path, Err: syscall.ESTALE}
			}
			var mounts []*mount.Info
			for _, mountpoint := range mountpoints {
				mounts = append(mounts, &mount.Info{Mountpoint: mountpoint, Root: "/"})
			}

			got, err := selectBindSource(poolInfo, mounts, target, stat)
			if wantErr {
				gomega.Expect(err).To(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			gomega.Expect(got).To(gomega.Equal(want))
		},
			ginkgo.Entry("consumer listed before the pool", []string{consumer, pool}, "", "", pool, false),
			ginkgo.Entry("pool listed before a consumer", []string{pool, consumer}, "", "", pool, false),
			ginkgo.Entry("stale consumer", []string{consumer, pool}, "", consumer, pool, false),
			ginkgo.Entry("only a consumer mounted", []string{consumer}, "", "", "", true),
			ginkgo.Entry("target mounts the pool", []string{pool, consumer, target}, pool, "", "", false),
			ginkgo.Entry("target mounts a consumer", []string{pool, consumer, target}, consumer, "", "", true),
			ginkgo.Entry("stale target", []string{pool, target}, pool, target, "", true),
		)
	})
})
