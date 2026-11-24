#!/bin/sh
#
# Check for less inclusive language usage.
# Allowed exceptions (hard to change) excluded with grep -v.
# Generated files excluded.
#

HPP_DIR="$(cd $(dirname $0)/../ && pwd -P)"

PHRASES='master|slave|whitelist|blacklist'

# Linux mount flags and mountinfo tags have fixed names. Allow their quoted
# strings only in the filesystem quota implementation and its tests.
VIOLATIONS=$(git grep -iI -E $PHRASES -- \
	':!vendor' \
	':!cluster-up' \
	':!cluster-sync' \
	':!*generated*' \
	':!*swagger.json*' \
	':!contrib/machineconfig-selinux-hpp.yaml' \
	':!hack/language.sh' \
		"${HPP_DIR}" \
		| grep -vE \
			-e 'github.com/Masterminds' \
			-e 'github.com/kubernetes' \
			-e 'golang/dep' \
			-e 'kubernetes-sigs/sig-storage-lib-external-provisioner' \
			-e '^cmd/mounter/filesystem_quota(_test)?\.go:.*"[^"]*--make-r?slave([ /"]|$)' \
			-e '^cmd/mounter/filesystem_quota(_test)?\.go:.*"[^"]*master:')
			# Allowed exceptions

if [ ! -z "${VIOLATIONS}" ]; then
	echo "ERROR: Found new additions of non-inclusive language ${PHRASES}"
	echo "${VIOLATIONS}"
	echo ""
	echo "Please consider different terminology if possible."
	echo "If necessary, an exception can be added to to the hack/language.sh script"
	exit 1
fi
