#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function log() {
    echo "# LOG: $1" >&3
}

function do_request() {
    HOST=${1:-localhost}
    log "Doing an initial request to Gateway on ${HOST}:${GATEWAY_FORWARDED_PORT}"
    curl --fail -kv --max-time 5 --resolve ${HOST}:${GATEWAY_FORWARDED_PORT}:127.0.0.1 "http://${HOST}:${GATEWAY_FORWARDED_PORT}"
}

function i2g() {
    MANIFEST="${1:-}"
    if [ -z "${MANIFEST}" ]; then
        echo "The source manifest is mandatory"
        exit 1
    fi
    log "Running i2g on e2e/manifests/${MANIFEST}"
    ${I2G} print --input-file e2e/manifests/"${MANIFEST}"
}