package helper

//HppOperatorClusterRole is a string yaml of the hpp operator cluster role
var HppOperatorClusterRole string = 
`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: hostpath-provisioner-operator
rules:
- apiGroups:
  - ""
  resources:
  - persistentvolumes
  verbs:
  - '*'
- apiGroups:
  - ""
  resources:
  - persistentvolumeclaims
  verbs:
  - get
  - list
  - watch
  - create
  - update
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - get
  - list
  - watch
  - create
  - patch
  - update
- apiGroups:
  - rbac.authorization.k8s.io
  resourceNames:
  - hostpath-provisioner
  - hostpath-provisioner-admin
  - hostpath-provisioner-admin-csi
  - hostpath-provisioner-health-check
  resources:
  - clusterrolebindings
  verbs:
  - update
  - delete
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - clusterrolebindings
  verbs:
  - list
  - get
  - watch
  - create
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - clusterroles
  verbs:
  - list
  - get
  - watch
  - create
- apiGroups:
  - rbac.authorization.k8s.io
  resourceNames:
  - hostpath-provisioner
  - hostpath-provisioner-admin
  - hostpath-provisioner-admin-csi
  - hostpath-provisioner-health-check
  resources:
  - clusterroles
  verbs:
  - update
  - delete
- apiGroups:
  - apps
  resourceNames:
  - hostpath-provisioner-operator
  resources:
  - deployments/finalizers
  verbs:
  - update
- apiGroups:
  - hostpathprovisioner.kubevirt.io
  resources:
  - '*'
  verbs:
  - '*'
- apiGroups:
  - security.openshift.io
  resources:
  - securitycontextconstraints
  verbs:
  - list
  - get
  - watch
  - create
- apiGroups:
  - security.openshift.io
  resourceNames:
  - hostpath-provisioner
  - hostpath-provisioner-csi
  resources:
  - securitycontextconstraints
  verbs:
  - delete
  - update
- apiGroups:
  - storage.k8s.io
  resources:
  - storageclasses
  verbs:
  - list
  - get
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - storage.k8s.io
  resources:
  - csinodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - storage.k8s.io
  resources:
  - csidrivers
  verbs:
  - list
  - create
  - get
  - watch
- apiGroups:
  - storage.k8s.io
  resourceNames:
  - kubevirt.io.hostpath-provisioner
  resources:
  - csidrivers
  verbs:
  - delete
  - update
- apiGroups:
  - storage.k8s.io
  resources:
  - volumeattachments
  verbs:
  - get
  - list
  - watch
  - patch
- apiGroups:
  - storage.k8s.io
  resources:
  - volumeattachments/status
  verbs:
  - patch
- apiGroups:
  - snapshot.storage.k8s.io
  resources:
  - volumesnapshotclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - snapshot.storage.k8s.io
  resources:
  - volumesnapshots
  verbs:
  - get
- apiGroups:
  - snapshot.storage.k8s.io
  resources:
  - volumesnapshotcontents
  verbs:
  - create
  - get
  - list
  - watch
  - update
  - delete
  - patch
- apiGroups:
  - snapshot.storage.k8s.io
  resources:
  - volumesnapshotcontents/status
  verbs:
  - update
  - patch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
`
