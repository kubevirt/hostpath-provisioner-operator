FROM registry.fedoraproject.org/fedora-minimal:34

RUN microdnf install -y xfsprogs

COPY _out/hostpath-provisioner-operator /usr/bin/hostpath-provisioner-operator
COPY _out/csv-generator /usr/bin/csv-generator
COPY _out/mounter /usr/bin/mounter
COPY _out/version.txt /version.txt
ENV PATH=/usr/bin
ENTRYPOINT ["hostpath-provisioner-operator"]
