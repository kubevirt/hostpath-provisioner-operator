# hostpath-provisioner-operator
The Kubernetes [operator](https://github.com/operator-framework) for managing [KubeVirt hostpath provisioner](https://github.com/kubevirt/hostpath-provisioner) deployment.
Leverages the [operator-sdk](https://github.com/operator-framework/operator-sdk/).

## How to deploy
Before deploying the operator, you need to create the hostpath provisioner namespace:
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/namespace.yaml
```
And then you can create the operator:
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/operator.yaml -n hostpath-provisioner
```

If you want to change the namespace in which you create the provisioner, make sure to update the ClusterRoleBinding and RoleBinding namespaces in the operator.yaml to match your namespace. Also change the namespace by changing the -n argument

Once you have installed the operator, you need to create an instance of the Custom Resource to deploy the hostpath provisioner in the hostpath-provisioner namespace.

### Custom Resource (CR)
[Example CR](deploy/hostpathprovisioner_cr.yaml) allows you specify the storage pool you wish to use as the backing storage for the persistent volumes. You specify the path to use to create volumes on the node, and the name of the storage pool. The name of the storage pool is used in the storage class to identify the pool.

(Once implemented) One can specify a storage class (and pvc template) that will be used to create PVC against, and those PVCs will be mounted by a pod that mounts the PV into the host on the specified path. Then the hostpath provisioner will created directories in that volume.

```yaml
apiVersion: hostpathprovisioner.kubevirt.io/v1beta1
kind: HostPathProvisioner
metadata:
  name: hostpath-provisioner
spec:
  imagePullPolicy: Always
  storagePools:
    - name: "local"
      path: "/var/hpvolumes"
  workload:
    nodeSelector:
      kubernetes.io/os: linux
```
This example names the storage pool 'local' and the path used is '/var/hpvolumes'. No storage class and pvc template is defined so the directories will be created on the node filesystem in the specified path. The [matching storage class](deploy/storageclass-wffc-csi.yaml) looks like this:
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: hostpath-csi
provisioner: kubevirt.io.hostpath-provisioner
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
parameters:
  storagePool: local
```
Notice the storagePool parameter. This lets the provisioner know which pool to use. Once multiple pools are supported you can create multiple storage classes, each pointing to a different pool.

#### Legacy CR
If you are using a previous version of the hostpath provisioner operator your CR will look like this:
```yaml
apiVersion: hostpathprovisioner.kubevirt.io/v1beta1
kind: HostPathProvisioner
metadata:
  name: hostpath-provisioner
spec:
  imagePullPolicy: IfNotPresent
  pathConfig:
    path: "/var/hpvolumes" #The path of the directory on the node
    useNamingPrefix: false #Use the name of the PVC bound to the created PV as part of the directory name.
```
The operator will continue to create the legacy provisioner in addition to the CSI driver. If you use the legacy format of the CR, you can use the [legacy CSI storage class](deploy/storageclass-wffc-legacy-csi.yaml) to create the storage class for the CSI driver.

To create the CustomResource
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/hostpathprovisioner_cr.yaml -n hostpath-provisioner
```
Once the CustomResource has been created, the operator will deploy the provisioner and CSI driver as a DaemonSet on each node.

### Storage Class
The hostpath provisioner supports two volumeBindingModes, Immediate and WaitForFirstConsumer. In general WaitForFirstConsumer is preferred however this requires Kubernetes >= 1.12 and if one is running an older kubernetes that volumeBindingMode will not work. Immediate binding mode is now *deprecated* and may be removed in the future. For this reason the operator will not create the StorageClass for you and you will have to do it yourself. Example storageclass yamls are available in [deploy](deploy) directory in this repository.

## SELinux (legacy only)
On each node you will have to give the directory you specify in the CR the appropriate selinux rules by running the following (assuming you pick /var/hpvolumes as your PathConfig path):
```bash
$ sudo chcon -t container_file_t -R /var/hpvolumes
```

Another way to configure SELinux when using OpenShift is using a [MachineConfig](./contrib/machineconfig-selinux-hpp.yaml).

## Deployment in OpenShift
The operator will create the appropriate SecurityContextConstraints for the hostpath provisioner to work and assign the ServiceAccount to that SCC. This operator will only work on OpenShift 4 and later (Kubernetes >= 1.12).
