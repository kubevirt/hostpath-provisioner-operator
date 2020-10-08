import json
import subprocess
import sys
import yaml
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('--storage_class', default="hostpath-provisioner")
parser.add_argument('--node', required=True)
parser.add_argument('--kubectl', default="kubectl")
parser.add_argument('--kubeconfig')

args = vars(parser.parse_args())
storage_class = args["storage_class"]
node_name = args["node"]
kubectl = args["kubectl"]
kubeconfig = args["kubeconfig"]

def main():
    pv_map = get_pvs_for_node(storage_class, node_name)
    # print(json.dumps(pvs[0], sort_keys=True, indent=4))
    #print(yaml.safe_dump(list(pv_map.values())[0], sys.stdout))
    #print("{} PVs".format(len(pv_map)))

    pvc_map = get_pvcs(pv_map)
    #print(yaml.safe_dump(list(pvc_map.values())[0], sys.stdout))
    #print("{} PVCs".format(len(pvc_map)))

    dv_map = get_dvs(pvc_map)
    #print(yaml.safe_dump(list(dv_map.values())[0], sys.stdout))
    #print("{} DataVolumes".format(len(dv_map)))

    vm_map = get_vms(dv_map)
    #print(yaml.safe_dump(list(vm_map.values())[0], sys.stdout))
    #print("{} VirtualMachines".format(len(vm_map)))

    data = {"pv": list(pv_map.values()), "pvc": list(pvc_map.values()),
            "dv": list(dv_map.values()), "vm": list(vm_map.values())}
    json.dump(data, sys.stdout, sort_keys=True, indent=4)


def get_vms(dv_map):
    result = {}
    for namespace, name in dv_map:
        dv = dv_map[(namespace, name)]
        owner_refs = dv["metadata"].get("ownerReferences", [])
        for owner_ref in owner_refs:
            if owner_ref["kind"] == "VirtualMachine" and owner_ref["controller"] == True:
                vm = kubectl_execute(
                    ["get", "vm", "-n", namespace, owner_ref["name"]])
                # namespace is stripped by export but we like it
                vm["metadata"]["namespace"] = namespace
                vm["spec"]["running"] = False
                result[(namespace, owner_ref["name"])] = vm
                for i, val in enumerate(vm["spec"]["dataVolumeTemplates"]):
                    val["spec"]["pvc"]["storageClassName"] = "hostpath-provisioner"
    return result


def get_dvs(pvc_map):
    result = {}
    for namespace, name in pvc_map:
        pvc = pvc_map[(namespace, name)]
        owner_refs = pvc["metadata"].get("ownerReferences", [])
        for owner_ref in owner_refs:
            if owner_ref["kind"] == "DataVolume" and owner_ref["controller"] == True:
                dv = kubectl_execute(
                    ["get", "dv", "-n", namespace, owner_ref["name"]])
                # namespace is stripped by export but we like it
                dv["metadata"]["namespace"] = namespace
                dv["spec"]["pvc"]["storageClassName"] = "hostpath-provisioner"
                result[(namespace, owner_ref["name"])] = dv
    return result


def get_pvcs(pv_map):
    result = {}
    for pv in pv_map.values():
        cr = pv["spec"].get("claimRef")
        if not cr:
            continue
        namespace, name = cr["namespace"], cr["name"]
        pvc = kubectl_execute(
            ["get", "pvc", "-n", namespace, name])
        # namespace is stripped by export but we like it
        pvc["metadata"]["namespace"] = namespace
        pvc["spec"]["storageClassName"] = "hostpath-provisioner"
        result[(namespace, name)] = pvc
    return result


def get_pvs_for_node(storage_class, node):
    all_pvs = kubectl_execute(["get", "pv"])
    result = {}
    for pv in all_pvs["items"]:
        if pv.get("spec", {}).get("storageClassName") != storage_class:
            continue
        nsts = pv.get("spec", {}).get("nodeAffinity", {}).get(
            "required", {}).get("nodeSelectorTerms", [])
        for nst in nsts:
            found = False
            mes = nst.get("matchExpressions", [])
            for me in mes:
                if me["key"] == "kubernetes.io/hostname" and node in me["values"]:
                    pv_export = kubectl_execute(
                        ["get", "pv", pv["metadata"]["name"]])
                    pv_export["spec"]["storageClassName"] = "hostpath-provisioner"
                    result[pv_export["metadata"]["name"]] = pv_export
                    found = True
                    break
            if found:
                break
    return result


def kubectl_execute(args):
    args = [kubectl] + args + ["-o", "json"]
    if kubeconfig:
        args = args + ["--kubeconfig", kubeconfig]
    p = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = p.communicate()
    if p.returncode != 0:
        sys.stdout.write("error executing {}".format(args))
        sys.stdout.write(stderr.decode())
        sys.exit(1)
    # print(stdout.decode())
    return json.loads(stdout)


if __name__ == '__main__':
    main()
