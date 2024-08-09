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
  - apps
  resources:
  - deployments
  verbs:
  - list
  - get
  - watch
  - create
  - delete
  - update
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - services
  verbs:
  - get
  - list
  - watch
  - create
- apiGroups:
  - ""
  resourceNames:
  - hpp-prometheus-metrics
  resources:
  - services
  verbs:
  - update
  - delete
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - create
  - get
- apiGroups:
  - ""
  resourceNames:
  - hostpath-provisioner-operator-lock
  resources:
  - configmaps
  verbs:
  - update
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
  - get
  - list
  - watch
  - update
  - create
  - delete
- apiGroups:
  - storage.k8s.io
  resources:
  - csistoragecapacities
  verbs:
  - get
  - list
  - watch
  - delete
  - update
  - create
- apiGroups:
  - monitoring.coreos.com
  resources:
  - servicemonitors
  - prometheusrules
  verbs:
  - list
  - get
  - watch
  - create
  - delete
  - update
  - patch
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
  - hostpath-provisioner-monitoring
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
  - hostpath-provisioner-monitoring
  resources:
  - roles
  verbs:
  - update
  - delete
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - create
  - delete
  - list
  - watch
`
