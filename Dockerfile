FROM registry.fedoraproject.org/fedora-minimal:39

RUN microdnf install -y xfsprogs util-linux

COPY _out/hostpath-provisioner-operator /usr/bin/hostpath-provisioner-operator
COPY _out/csv-generator /usr/bin/csv-generator
COPY _out/mounter /usr/bin/mounter
COPY _out/version.txt /version.txt
USER 1000
ENV PATH=/usr/bin
ENTRYPOINT ["hostpath-provisioner-operator"]
