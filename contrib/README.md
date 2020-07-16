# hostpath-provisioner-upgrade
This REPO houses the scripts needed to export VMs/DataVolumes/PVC/PVs(objects) from an OpenShift 3.11 + CNV 1.4.1 cluster to an OpenShift 4.x cluster with CNV 2.2+. The scripts assume that the nodes from 3.11 will be repurposed as 4.x nodes after the objects have been exported. It also assumes that the data directories will be preserved when moving the node from 3.11 to 4.x. The scripts require that python 3 is installed on the machine running the scripts, and that the user running the script has admin rights on both clusters.

The scripts are designed to work on a node by node basis, capturing all the objects from one node to exported.

## Exporting
To run the script do the following:
```bash
$ ./export.sh <node_name1> <node_name2> ...
```
You must specify at least one node name. If more than one is specified the script will loop over each node and create a directory for each node. It will export the PVs on that node and then export the matching PVC and DV. Then it will export the VMs that use those DVs. There will be a file called export.json in the export directory that contains the objects in json format. You can specify your kubectl by setting the KUBECTL environment variable. You can also specify the KUBECONFIG environment variable to get correct access to the cluster. if not set it defaults to kubectl. You can also specify the storage class name of the hostpath provisioner that you are using. if not set it defaults to hostpath-provisioner (earlier versions of the hostpath provisioner had it named kubevirt-hostpath-provisioner).

The script will verify that you have python 3 installed and that the node you specified exists in the cluster before proceeding with the export.

## Importing
To run the script do the following:
```bash
./import.sh <node_name1> <node_name2> ...
```

This will read the export.json file in the node_name directory for each specified node_name, and attempt to import the objects into the cluster putting the PVs on the node specified, the storage class will be the same as the original cluster. Both KUBECTL and KUBECONFIG environment variables can be set for this script as well. There are additional environment variables that can be set to specify the namespace of kubevirt and cdi. In order to work, the script has to disable the kubevirt and cdi controllers (otherwise those will stop the script from creating data volumes and pvcs properly). The default namespace is kubevirt and cdi, in a cnv environment these will be set to openshift-cnv, so you must specify the right KUBEVIRT_NS and CDI_NS environment variable names.

