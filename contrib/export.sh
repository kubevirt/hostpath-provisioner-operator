#!/bin/bash 
set -e

KUBECTL=${KUBECTL:-"kubectl"}

function _kubectl(){
  $KUBECTL "$@"
}

STORAGE_CLASS=${STORAGE_CLASS:-"kubevirt-hostpath-provisioner"}

if [ $# -eq 0 ]; then
    echo "No nodes provided, aborting"
    exit 1
fi

if command -v python3 &>/dev/null; then
    echo Python 3 is installed, continuing
else
    echo Python 3 is not installed, this script requires python.
fi

# check if all nodes exist.
for node in "$@"
do
  echo "Checking node: ${node}"
  #check if node exists.
  r=$(_kubectl get node ${node})
  echo "Found node $node"
done

# loop over nodes, presumed to be arguments.
for node in "$@"
do
  echo "Creating output directory for node $node"
  mkdir -p ./$node

  set +e
  if [ -z $KUBECONFIG ]; then
      python3 export_pv.py --storage_class $STORAGE_CLASS --kubectl $KUBECTL --node $node > ./$node/export.json
  else
      python3 export_pv.py --storage_class $STORAGE_CLASS --kubectl $KUBECTL --kubeconfig $KUBECONFIG --node $node > ./$node/export.json
  fi
  (( $? != 0 )) && echo "Export failed, ${node}/export.json is corrupt" && cat ./$node/export.json
  set -e
done
