#!/bin/bash -e

KUBEVIRT_NS=${KUBEVIRT_NS:-kubevirt}
CDI_NS=${CDI_NS:-cdi}
KUBECTL=${KUBECTL:-"kubectl"}

function _kubectl(){
  $KUBECTL "$@"
}

if [ $# -eq 0 ]; then
    echo "No nodes provided, aborting"
    exit 1
fi

if command -v python3 &>/dev/null; then
    echo Python 3 is installed, continuing
else
    echo Python 3 is not installed, this script requires python.
fi

echo "Kubevirt namespace $KUBEVIRT_NS, CDI namespace $CDI_NS"

for node in "$@"
do
  echo "Checking node: ${node}"
  #check if node exists.
  r=$(_kubectl get node ${node})
  echo "Found node $node"
  echo "Verifying input directory exists."
  if [ ! -d "./$node" ]; then
    echo "Directory $node DOES NOT exist."
    exit 1
  fi
  echo "Verifying input file exists."
  if [ ! -f "./$node/export.json" ]; then
    echo "Data file DOES NOT exist in $node."
    exit 1
  fi
  echo "Verifying \"dataSource\": null is not in file"
  if grep -q "\"dataSource\": null" "./$node/export.json"; then
    echo "Corrupted export file $node/export.json detected, it contains '\"dataSource\": null'"
    exit 1
  fi
done

orgCvoReplicas=$(_kubectl get deployment cluster-version-operator -n openshift-cluster-version -o jsonpath="{@.spec.replicas}")
echo "Current CVO requested replicas: $orgCvoReplicas"
echo "Bringing down CVO and OLM, warning this will generate cluster health alerts!!!"
_kubectl scale -n openshift-cluster-version deployment/cluster-version-operator --replicas=0
cvoReplicas=$(_kubectl get deployment cluster-version-operator -n openshift-cluster-version -o jsonpath="{@.status.readyReplicas}")
while (( 0 != cvoReplicas )); do
  sleep 5;
  cvoReplicas=$(_kubectl get deployment cluster-version-operator -n openshift-cluster-version -o jsonpath="{@.status.readyReplicas}")
done

orgOlmReplicas=$(_kubectl get deployment olm-operator -n openshift-operator-lifecycle-manager -o jsonpath="{@.spec.replicas}")
echo "Current OLM requested replicas: $orgOlmReplicas"
_kubectl scale -n openshift-operator-lifecycle-manager deployment/olm-operator --replicas=0
omlReplicas=$(_kubectl get deployment olm-operator -n openshift-operator-lifecycle-manager -o jsonpath="{@.status.readyReplicas}")
while (( 0 != omlReplicas )); do
  sleep 5;
  omlReplicas=$(_kubectl get deployment olm-operator -n openshift-operator-lifecycle-manager -o jsonpath="{@.status.readyReplicas}")
done

echo "Verified CVO and OLM are down, bringing down kubevirt and CDI"
# D/S namespace is openshift-cnv I believe, but testing here with kubevirt and cdi
orgKubevirtOperatorReplicas=$(_kubectl get deployment virt-operator -n $KUBEVIRT_NS -o jsonpath="{@.spec.replicas}")
echo "Current Kubevirt operator requested replicas: $orgKubevirtOperatorReplicas"
_kubectl scale -n $KUBEVIRT_NS deployment/virt-operator --replicas=0
kubevirtOperatorReplicas=$(_kubectl get deployment virt-operator -n $KUBEVIRT_NS -o jsonpath="{@.status.readyReplicas}")
while (( 0 != kubevirtOperatorReplicas )); do
  sleep 5;
  kubevirtOperatorReplicas=$(_kubectl get deployment virt-operator -n $KUBEVIRT_NS -o jsonpath="{@.status.readyReplicas}")
done

orgKubevirtControllerReplicas=$(_kubectl get deployment virt-controller -n $KUBEVIRT_NS -o jsonpath="{@.spec.replicas}")
echo "Current virt controller requested replicas: $orgKubevirtControllerReplicas"
_kubectl scale -n $KUBEVIRT_NS deployment/virt-controller --replicas=0
kubevirtControllerReplicas=$(_kubectl get deployment virt-controller -n $KUBEVIRT_NS -o jsonpath="{@.status.readyReplicas}")
while (( 0 != kubevirtControllerReplicas )); do
  sleep 5;
  kubevirtControllerReplicas=$(_kubectl get deployment virt-controller -n $KUBEVIRT_NS -o jsonpath="{@.status.readyReplicas}")
done

orgCDIOperatorReplicas=$(_kubectl get deployment cdi-operator -n $CDI_NS -o jsonpath="{@.spec.replicas}")
echo "Current CDI operator requested replicas: $orgCDIOperatorReplicas"
_kubectl scale -n $CDI_NS deployment/cdi-operator --replicas=0
cdiOperatorReplicas=$(_kubectl get deployment cdi-operator -n $CDI_NS -o jsonpath="{@.status.readyReplicas}")
while (( 0 != cdiOperatorReplicas )); do
  sleep 5;
  cdiOperatorReplicas=$(_kubectl get deployment cdi-operator -n $CDI_NS -o jsonpath="{@.status.readyReplicas}")
done

orgCDIControllerReplicas=$(_kubectl get deployment cdi-deployment -n $CDI_NS -o jsonpath="{@.spec.replicas}")
echo "Current CDI controller requested replicas: $orgCDIControllerReplicas"
_kubectl scale -n $CDI_NS deployment/cdi-deployment --replicas=0
cdiControllerReplicas=$(_kubectl get deployment cdi-deployment -n $CDI_NS -o jsonpath="{@.status.readyReplicas}")
while (( 0 != cdiControllerReplicas )); do
  sleep 5;
  cdiControllerReplicas=$(_kubectl get deployment cdi-deployment -n $CDI_NS -o jsonpath="{@.status.readyReplicas}")
done

echo "Kubevirt and CDI are down, importing Virtual Machines"
#don't exit on failure, we need to restore
set +e
for node in "$@"
do
  if [ -z $KUBECONFIG ]; then
    python3 import_pv.py --kubectl $KUBECTL < ./$node/export.json
  else
    python3 import_pv.py --kubectl $KUBECTL --kubeconfig $KUBECONFIG < ./$node/export.json
  fi
  (( $? != 0 )) && echo "Import failed"
done
(( $? == 0 )) && echo "Finished importing"
_kubectl scale -n $CDI_NS deployment/cdi-deployment --replicas=$orgCDIControllerReplicas
_kubectl scale -n $CDI_NS deployment/cdi-operator --replicas=$orgCDIOperatorReplicas
_kubectl scale -n $KUBEVIRT_NS deployment/virt-controller --replicas=$orgKubevirtControllerReplicas
_kubectl scale -n $KUBEVIRT_NS deployment/virt-operator --replicas=$orgKubevirtOperatorReplicas
echo "Kubevirt and CDI restored"
_kubectl scale -n openshift-operator-lifecycle-manager deployment/olm-operator --replicas=$orgOlmReplicas
_kubectl scale -n openshift-cluster-version deployment/cluster-version-operator --replicas=$orgCvoReplicas
echo "Finished restoring cluster operations"

