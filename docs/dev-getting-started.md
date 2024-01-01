# Developer Getting Started

A quick start guide to test changes by running hostpath-provisioner-operator using hostpath-provisioner cluster.

## Test a pr using hostpath-provisioner

Clone the hpp repository
```bash
$ git clone https://github.com/kubevirt/hostpath-provisioner.git
$ cd hostpath-provisioner
```
**Optional**: export non-default deployments, for example Prometheus 
```bash
$ export KUBEVIRT_DEPLOY_PROMETHEUS=true
$ export KUBEVIRT_DEPLOY_PROMETHEUS_ALERTMANAGER=true
````
Create the cluster
```bash
$ make cluster-up
$ make-cluster sync
```
Check and save the `registry port` for next steps
```bash
$ ./cluster-up/cli.sh ports registry
```

Clone the hpp-operator repository
```bash
$ git clone https://github.com/kubevirt/hostpath-provisioner-operator.git
$ cd hostpath-provisioner-operator
#checkout your local changes
$ git checkout <your-branch>
```

Set the local `registry port` from earlier step as the docker repo, and disable TLS verify
```bash
$ export DOCKER_REPO=localhost:<registry-port>
$ export BUILDAH_TLS_VERIFY=false
```

In order to test local changes update the built image to use the local docker image instead of latest
```bash
$ vi deploy/operator.yaml
# look for these lines:
# Replace this with the built image name
# image: quay.io/kubevirt/hostpath-provisioner-operator:latest
#and update to:
# image: registry:5000/hostpath-provisioner-operator:latest
$ make push
```

Now we can re-deploy the hpp-operator with our changes on the hpp cluster
```bash
$ cd hostpath-provisioner
$ alias k='./cluster-up/kubectl.sh'
$ k delete -f ../hostpath-provisioner-operator/deploy/hostpathprovisioner_cr.yaml
$ k delete -f ../hostpath-provisioner-operator/deploy/operator.yaml -n hostpath-provisioner
$ k apply -f ../hostpath-provisioner-operator/deploy/operator.yaml -n hostpath-provisioner
# wait for the operator deploy to be finished before applying the cr again
$ k rollout status -n hostpath-provisioner deployment/hostpath-provisioner-operator --timeout=120s
$ k apply -f ../hostpath-provisioner-operator/deploy/hostpathprovisioner_cr.yaml
```

**Note:** You will need to repeat the make push and the re-deploy steps to test changes made after push.
