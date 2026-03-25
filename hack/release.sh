#!/usr/bin/env bash

set -exuo pipefail

function cleanup_gh_install() {
    [ -n "${gh_cli_dir}" ] && [ -d "${gh_cli_dir}" ] && rm -rf "${gh_cli_dir:?}/"
}

function ensure_gh_cli_installed() {
    if command -V gh; then
        return
    fi

    trap 'cleanup_gh_install' EXIT SIGINT SIGTERM

    # install gh cli for uploading release artifacts, with prompt disabled to enforce non-interactive mode
    gh_cli_dir=$(mktemp -d)
    (
        cd  "$gh_cli_dir/"
        curl -sSL "https://github.com/cli/cli/releases/download/v${GH_CLI_VERSION}/gh_${GH_CLI_VERSION}_linux_amd64.tar.gz" -o "gh_${GH_CLI_VERSION}_linux_amd64.tar.gz"
        tar xvf "gh_${GH_CLI_VERSION}_linux_amd64.tar.gz"
    )
    export PATH="$gh_cli_dir/gh_${GH_CLI_VERSION}_linux_amd64/bin:$PATH"
    if ! command -V gh; then
        echo "gh cli not installed successfully"
        exit 1
    fi
    gh config set prompt disabled
}

function update_github_release() {
    # note: for testing purposes we set the target repository, gh cli seems to always automatically choose the
    # upstream repository automatically, even when you are in a fork

    set +e
    if ! gh release view --repo "$GITHUB_REPOSITORY" "$TAG" ; then
        set -e
        gh release create --repo "$GITHUB_REPOSITORY" "$TAG" --title="$TAG"
    else
        set -e
    fi

    # Create a temporary directory for modified deployment files
    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    # Copy all yaml files to temp directory
    cp deploy/*.yaml "$temp_dir/"

    # Update operator.yaml image tags to match the release tag (replace both 'latest' and version tags)
    sed -i "s|quay.io/kubevirt/hostpath-provisioner-operator:\(latest\|v[0-9.]*\)|quay.io/kubevirt/hostpath-provisioner-operator:${TAG}|g" "$temp_dir/operator.yaml"
    sed -i "s|quay.io/kubevirt/hostpath-provisioner:\(latest\|v[0-9.]*\)|quay.io/kubevirt/hostpath-provisioner:${TAG}|g" "$temp_dir/operator.yaml"
    sed -i "s|quay.io/kubevirt/hostpath-csi-driver:\(latest\|v[0-9.]*\)|quay.io/kubevirt/hostpath-csi-driver:${TAG}|g" "$temp_dir/operator.yaml"

    gh release upload --repo "$GITHUB_REPOSITORY" --clobber "$TAG" \
        "$temp_dir"/*.yaml
}

function main() {
    TAG="$(git tag --points-at HEAD | head -1)"
    if [ -z "$TAG" ]; then
        echo "commit $(git show -s --format=%h) doesn't have a tag, exiting..."
        exit 0
    fi

    export TAG

    GIT_ASKPASS="$(pwd)/hack/git-askpass.sh"
    [ -f "$GIT_ASKPASS" ] || exit 1
    export GIT_ASKPASS

    ensure_gh_cli_installed

    gh auth login --with-token <"$GITHUB_TOKEN_PATH"

    update_github_release
}

main "$@"

