#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

BOOTKUBE_ROOT=${SCRIPT_DIR}/..
BOOTKUBE_RELEASE=${BOOTKUBE_BIN:-${BOOTKUBE_ROOT}/_output/release/bootkube-linux-amd64.tar.gz}
DOCKER_REPO=${DOCKER_REPO:-quay.io/coreos/bootkube}
DOCKER_TAG=${DOCKER_TAG:-$(${BOOTKUBE_ROOT}/build/git-version.sh)}
DOCKER_PUSH=${DOCKER_PUSH:-false}
TARGET=${TARGET:-clean release}

sudo rkt run \
    --volume bk,kind=host,source=${BOOTKUBE_ROOT} \
    --mount volume=bk,target=/go/src/github.com/coreos/bootkube \
    --insecure-options=image docker://golang:1.6.2 --exec /bin/bash -- -c \
    "cd /go/src/github.com/coreos/bootkube && make ${TARGET}"

TEMPDIR=$(mktemp -d -t bootkube.XXXX)

cp ${BOOTKUBE_ROOT}/Dockerfile ${TEMPDIR}/Dockerfile
tar xzvf ${BOOTKUBE_RELEASE} -C ${TEMPDIR}
docker build -t ${DOCKER_REPO}:${DOCKER_TAG} -f ${TEMPDIR}/Dockerfile ${TEMPDIR}
rm -rf ${TEMPDIR}

if [[ ${DOCKER_PUSH} == "true" ]]; then
    docker push ${DOCKER_REPO}:${DOCKER_TAG}
fi
