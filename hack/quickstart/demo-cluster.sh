#!/bin/bash
set -euo pipefail

# CREATE CLUSTER:
#
# ./demo-cluster.sh <cluster-name>
#
# CLEAN UP CLUSTER:
#
# CLEANUP=true ./demo-cluster.sh <cluster-name>

WORKER_COUNT=${WORKER_COUNT:-1}
COREOS_IMAGE=${COREOS_IMAGE:-'https://www.googleapis.com/compute/v1/projects/coreos-cloud/global/images/coreos-stable-1122-2-0-v20160906'}
CLEANUP=${CLEANUP:-"false"}

BOOTKUBE_ROOT=$(git rev-parse --show-toplevel)
CLUSTER=${1:-}
CLUSTER_DIR=${CLUSTER_DIR:-${CLUSTER}}

function add_master {
    gcloud compute instances create ${CLUSTER}-master-1 \
        --image ${COREOS_IMAGE} --zone us-central1-a --machine-type n1-standard-1 --boot-disk-size=10GB

    gcloud compute instances add-tags --zone us-central1-a ${CLUSTER}-master-1 --tags ${CLUSTER}-apiserver
    gcloud compute firewall-rules create ${CLUSTER}-api-443 --target-tags=${CLUSTER}-apiserver --allow tcp:443

    MASTER_IP=$(gcloud compute instances list ${CLUSTER}-master-1 --format=json | jq --raw-output '.[].networkInterfaces[].accessConfigs[].natIP')
    cd ${BOOTKUBE_ROOT}/hack/quickstart && CLUSTER_DIR=${CLUSTER_DIR} IDENT=~/.ssh/google_compute_engine SSH_OPTS="-o StrictHostKeyChecking=no" ./init-master.sh ${MASTER_IP}
}

function add_workers {
    for i in $(seq 1 ${WORKER_COUNT}); do
        gcloud compute instances create ${CLUSTER}-worker-${i} \
            --image ${COREOS_IMAGE} --zone us-central1-a --machine-type n1-standard-1

        local WORKER_IP=$(gcloud compute instances list ${CLUSTER}-worker-${i} --format=json | jq --raw-output '.[].networkInterfaces[].accessConfigs[].natIP')
        cd ${BOOTKUBE_ROOT}/hack/quickstart && IDENT=~/.ssh/google_compute_engine SSH_OPTS="-o StrictHostKeyChecking=no" ./init-worker.sh ${WORKER_IP} ${CLUSTER_DIR}/auth/kubeconfig
    done
}

function cleanup {
    gcloud compute instances delete --quiet --zone us-central1-a ${CLUSTER}-master-1 || true
    gcloud compute firewall-rules delete --quiet ${CLUSTER}-api-443 || true
    for i in $(seq 1 ${WORKER_COUNT}); do
        gcloud compute instances delete --quiet --zone us-central1-a ${CLUSTER}-worker-${i} || true
    done
    rm -rf ${CLUSTER_DIR}
}

if [ -z "${CLUSTER}" ]; then
    echo "USAGE: $0 <cluster-name>"
    exit 1
fi

if [ "${CLEANUP}" = "true" ]; then
    cleanup
    exit 0
else
    add_master
    add_workers
    echo
    echo "Done with demo-cluster setup."
    echo "To remove this cluster run: CLEANUP=true $0 ${CLUSTER}"
    echo
    echo "To use kubectl set your kubeconfig path. Example:"
    echo "kubectl --kubeconfig=${CLUSTER_DIR}/auth/kubeconfig get nodes"
fi
