package helper

//HppOperatorDeployment is a string yaml of the hpp operator deployment
var HppOperatorDeployment string = 
`apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    prometheus.hostpathprovisioner.kubevirt.io: "true"
  name: hostpath-provisioner-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      name: hostpath-provisioner-operator
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        name: hostpath-provisioner-operator
        prometheus.hostpathprovisioner.kubevirt.io: "true"
    spec:
      containers:
      - command:
        - hostpath-provisioner-operator
        env:
        - name: WATCH_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: INSTALLER_PART_OF_LABEL
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['app.kubernetes.io/part-of']
        - name: INSTALLER_VERSION_LABEL
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['app.kubernetes.io/version']
        - name: OPERATOR_NAME
          value: hostpath-provisioner-operator
        - name: OPERATOR_IMAGE
          value: quay.io/kubevirt/hostpath-provisioner-operator:latest
        - name: PROVISIONER_IMAGE
          value: quay.io/kubevirt/hostpath-provisioner:latest
        - name: CSI_PROVISIONER_IMAGE
          value: quay.io/kubevirt/hostpath-csi-driver:latest
        - name: NODE_DRIVER_REG_IMAGE
          value: k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.2.0
        - name: LIVENESS_PROBE_IMAGE
          value: k8s.gcr.io/sig-storage/livenessprobe:v2.3.0
        - name: CSI_SNAPSHOT_IMAGE
          value: k8s.gcr.io/sig-storage/csi-snapshotter:v4.2.1
        - name: CSI_SIG_STORAGE_PROVISIONER_IMAGE
          value: k8s.gcr.io/sig-storage/csi-provisioner:v2.2.1
        - name: VERBOSITY
          value: "3"
        - name: MONITORING_NAMESPACE
        image: quay.io/kubevirt/hostpath-provisioner-operator:latest
        imagePullPolicy: Always
        livenessProbe:
          failureThreshold: 1
          httpGet:
            path: /livez
            port: 6060
            scheme: HTTP
          initialDelaySeconds: 30
          periodSeconds: 5
        name: hostpath-provisioner-operator
        ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /readyz
            port: 6060
            scheme: HTTP
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 10m
            memory: 150Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          runAsNonRoot: true
          seccompProfile:
            type: RuntimeDefault
        volumeMounts:
        - mountPath: /tmp/k8s-webhook-server/serving-certs
          name: apiservice-cert
      serviceAccountName: hostpath-provisioner-operator
      volumes:
      - name: apiservice-cert
        secret:
          defaultMode: 420
          items:
          - key: tls.crt
            path: tls.crt
          - key: tls.key
            path: tls.key
          secretName: hostpath-provisioner-operator-webhook-service-cert
status: {}
`
