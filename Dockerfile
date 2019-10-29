FROM registry.fedoraproject.org/fedora-minimal:30
COPY _out/hostpath-provisioner-operator /hostpath-provisioner-operator
COPY _out/version.txt /version.txt
ENV PATH=/
ENTRYPOINT ["/hostpath-provisioner-operator"]
