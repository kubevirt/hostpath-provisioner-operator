# hostpath-provisioner-operator
The kubernetes [operator](https://github.com/operator-framework) for managing [Kubevirt hostpath provisioner](https://github.com/kubevirt/hostpath-provisioner) deployment.
Leverages the [operator-sdk](https://github.com/operator-framework/operator-sdk/).

## How to deploy
In order to deploy the operator you have to install several yaml files for the operator to deploy.
- [ClusterRole](deploy/cluster_role.yaml)
- [ClusterRoleBinding](deploy/cluster_role_binding.yaml)
- [ServiceAccount](deploy/service_account.yaml)
- [Operator](deploy/operator.yaml)
- [CRD](deploy/crds/hostpathprovisioner.kubevirt.io_hostpathprovisioners_crd.yaml)

Once you have installed the yamls, you need to create an instance of the Custom Resource to deploy the hostpath provisioner.
### Custom Resource (CR)
[Example CR](deploy/crds/hostpathprovisioner_cr.yaml) allows you specify the directory you wish to use as the backing directory for the persistent volumes. You can also specify if you wish to use the name of the PersistentVolume as part of the directory that is created by the provisioner. All the values in the spec are required.
```yaml
apiVersion: hostpathprovisioner.kubevirt.io/v1alpha1
kind: HostPathProvisioner
metadata:
  name: hostpath-provisioner
spec:
  imagePullPolicy: IfNotPresent
  imageRegistry: kubevirt/hostpath-provisioner #Registry to get the hostpath provisioner container from
  imageTag: latest #Tag of the hostpath provisioner container
  pathConfig:
    path: "/var/hpvolumes" #The path of the directory on the node
    useNamingPrefix: "false" #Use the name of the PVC bound to the created PV as part of the directory name.
```

Note the yamls assume you want to deploy into the hostpath_provisioner namespace, which has to be created ahead of time. If you want to install into a different namespace you will have modify the ClusterRoleBinding namespace to match. The rest can be installed using -n to specify the namespace.

## Selinux
On each node you will have to give the directory you specify in the CR the appropriate selinx rules by running the following (assuming you pick /var/hpvolumes as your PathConfig path):
```bash
$ sudo chcon -R unconfined_u:object_r:svirt_sandbox_file_t:s0 /var/hpvolumes
```

## Deployment in OpenShift
The operator will create the appropriate SecurityContextConstraints for the hostpath provisioner to work and assign the ServiceAccount to that SCC. 
