# hostpath-provisioner-operator
The Kubernetes [operator](https://github.com/operator-framework) for managing [KubeVirt hostpath provisioner](https://github.com/kubevirt/hostpath-provisioner) deployment.
Leverages the [operator-sdk](https://github.com/operator-framework/operator-sdk/).

## How to deploy
Before deploying the operator, you need to create the hostpath provisioner namespace:
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/master/deploy/namespace.yaml
```
And then you can create the operator:
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/master/deploy/operator.yaml -n hostpath-provisioner
```

If you want to change the namespace in which you create the provisioner, make sure to update the ClusterRoleBinding and RoleBinding namespaces in the operator.yaml to match your namespace. Also change the namespace by changing the -n argument

Once you have installed the operator, you need to create an instance of the Custom Resource to deploy the hostpath provisioner in the hostpath-provisioner namespace.

### Custom Resource (CR)
[Example CR](deploy/hostpathprovisioner_cr.yaml) allows you specify the directory you wish to use as the backing directory for the persistent volumes. You can also specify if you wish to use the name of the PersistentVolume as part of the directory that is created by the provisioner. All the values in the spec are required.
```yaml
apiVersion: hostpathprovisioner.kubevirt.io/v1alpha1
kind: HostPathProvisioner
metadata:
  name: hostpath-provisioner
spec:
  imagePullPolicy: IfNotPresent
  imageRegistry: quay.io/kubevirt/hostpath-provisioner #Registry to get the hostpath provisioner container from
  imageTag: latest #Tag of the hostpath provisioner container
  pathConfig:
    path: "/var/hpvolumes" #The path of the directory on the node
    useNamingPrefix: "false" #Use the name of the PVC bound to the created PV as part of the directory name.
```

To create the CustomResource
```bash
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/master/deploy/hostpathprovisioner_cr.yaml -n hostpath-provisioner
```
Once the CustomResource has been created, the operator will deploy the provisioner as a DaemonSet on each node.

## SELinux
On each node you will have to give the directory you specify in the CR the appropriate selinux rules by running the following (assuming you pick /var/hpvolumes as your PathConfig path):
```bash
$ sudo chcon -t container_file_t -R /var/hpvolumes
```

## Deployment in OpenShift
The operator will create the appropriate SecurityContextConstraints for the hostpath provisioner to work and assign the ServiceAccount to that SCC. This operator will only work on OpenShift 4 and later (Kubernetes >= 1.12).
