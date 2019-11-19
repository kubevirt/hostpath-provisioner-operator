FROM registry.fedoraproject.org/fedora-minimal:30
COPY _out/hostpath-provisioner-operator /usr/bin/hostpath-provisioner-operator
COPY _out/csv-generator /usr/bin/csv-generator
COPY _out/version.txt /version.txt
ENV PATH=/usr/bin
ENTRYPOINT ["hostpath-provisioner-operator"]
