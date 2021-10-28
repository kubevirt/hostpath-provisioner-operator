package helper

//HppOperatorRole is a string yaml of the hpp operator role
var HppOperatorRole string = 
`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: hostpath-provisioner-operator
rules:
- apiGroups:
  - apps
  resources:
  - daemonsets
  verbs:
  - list
  - get
  - watch
  - create
- apiGroups:
  - apps
  resourceNames:
  - hostpath-provisioner
  - hostpath-provisioner-csi
  resources:
  - daemonsets
  verbs:
  - delete
  - update
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - create
  - get
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - list
  - get
  - create
  - watch
- apiGroups:
  - ""
  resourceNames:
  - hostpath-provisioner-admin
  - hostpath-provisioner-admin-csi
  resources:
  - serviceaccounts
  verbs:
  - update
  - delete
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - '*'
- apiGroups:
  - storage.k8s.io
  resources:
  - csistoragecapacities
  verbs:
  - '*'
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - rolebindings
  verbs:
  - list
  - get
  - watch
  - create
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - roles
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
  - rolebindings
  verbs:
  - update
  - delete
- apiGroups:
  - rbac.authorization.k8s.io
  resourceNames:
  - hostpath-provisioner
  - hostpath-provisioner-admin
  - hostpath-provisioner-admin-csi
  - hostpath-provisioner-health-check
  resources:
  - roles
  verbs:
  - update
  - delete
`
