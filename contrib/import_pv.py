import json
import subprocess
import sys
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('--kubectl', default="kubectl")
parser.add_argument('--kubeconfig')

args = vars(parser.parse_args())
kubectl = args["kubectl"]
kubeconfig = args["kubeconfig"]

def main():
    data = json.load(sys.stdin)

    vm_map = create_vms(data["vm"])
    print("Created {} VirtualMachines".format(len(vm_map)))

    dv_map = create_dvs(vm_map, data["dv"])
    print("Created {} DataVolumes".format(len(dv_map)))

    pvc_map = create_pvcs(dv_map, data["pvc"])
    print("Created {} PVCs".format(len(pvc_map)))

    pv_map = create_pvs(data["pv"])
    print("Created {} PVs".format(len(pv_map)))


def create_pvs(pv_list):
    result = {}
    for pv in pv_list:
        if "claimRef" in pv["spec"]:
            del pv["spec"]["claimRef"]
        input = json.dumps(pv).encode("utf-8")
        new_pv = kubectl_execute(
            ["create", "-f", "-"], json_output=True, input=input)
        result[pv["metadata"]["name"]] = new_pv
    return result


def create_pvcs(dv_map, pvc_list):
    result = {}
    for pvc in pvc_list:
        namespace, name = pvc["metadata"]["namespace"], pvc["metadata"]["name"]
        owner_refs = pvc["metadata"].get("ownerReferences", [])
        for owner_ref in owner_refs:
            if owner_ref["kind"] == "DataVolume":
                dv = dv_map[(namespace, owner_ref["name"])]
                owner_ref["uid"] = dv["metadata"]["uid"]
                break
        input = json.dumps(pvc).encode("utf-8")
        new_pvc = kubectl_execute(
            ["create", "-f", "-"], json_output=True, input=input)
        result[(namespace, name)] = new_pvc
    return result


def create_dvs(vm_map, dv_list):
    result = {}
    for dv in dv_list:
        namespace, name = dv["metadata"]["namespace"], dv["metadata"]["name"]
        owner_refs = dv["metadata"].get("ownerReferences", [])
        for owner_ref in owner_refs:
            if owner_ref["kind"] == "VirtualMachine":
                vm = vm_map[(namespace, owner_ref["name"])]
                owner_ref["uid"] = vm["metadata"]["uid"]
                break
        input = json.dumps(dv).encode("utf-8")
        new_dv = kubectl_execute(
            ["create", "-f", "-"], json_output=True, input=input)
        result[(namespace, name)] = new_dv
    return result


def create_vms(vm_list):
    result = {}
    for vm in vm_list:
        input = json.dumps(vm).encode("utf-8")
        new_vm = kubectl_execute(
            ["create", "-f", "-"], json_output=True, input=input)
        namespace, name = new_vm["metadata"]["namespace"], new_vm["metadata"]["name"]
        result[(namespace, name)] = new_vm
    return result


def kubectl_execute(args, json_output=False, input=None):
    new_args = [kubectl]
    if json_output:
        new_args += ["-o", "json"]
    new_args += args
    if kubeconfig:
        new_args += ["--kubeconfig", kubeconfig]
    p = subprocess.Popen(new_args, stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, stdin=subprocess.PIPE)
    stdout, stderr = p.communicate(input=input)
    if p.returncode != 0:
        print("error executing {}".format(args))
        print(stderr.decode())
        sys.exit(1)
    # print(stdout.decode())
    if json_output:
        return json.loads(stdout)
    return stdout.decode()


if __name__ == '__main__':
    main()
