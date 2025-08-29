#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

export GATEWAY_FORWARDED_PORT="${GATEWAY_PORT:-8888}"

source e2e/tests/functions.bash

# TODO: We may want to accept other providers, for now to keep it simple we will go with Envoy Gateway
function setup_suite {
    log "Installing envoy gateway"

    kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.5.0/install.yaml
    kubectl wait --timeout=5m -n envoy-gateway-system deployment/envoy-gateway --for=condition=Available

    # Install envoy gateway class
    log "Creating GatewayClass"
    kubectl apply -f e2e/manifests/infrastructure/gatewayclass.yaml
    kubectl wait gatewayclass/eg --for=condition=Accepted=True
    # TODO: Add HTTPS listener
    log "Creating Gateway"
    kubectl apply -f e2e/manifests/infrastructure/gateway.yaml
    sleep 3 # Give some time for controller to reconcile gateway and start a deployment
    export GATEWAY_NAME=$(kubectl get deploy -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=eg -o name |cut -f 2 -d /)
    kubectl wait --timeout=2m deploy -n envoy-gateway-system ${GATEWAY_NAME} --for=condition=Available
    log "Starting port-forwarding to gateway"
    kubectl -n envoy-gateway-system port-forward service/${GATEWAY_NAME} ${GATEWAY_FORWARDED_PORT}:80 &
    export KUBECTL_FORWARDER_PID=$!

    log "Setting the sample application"
    kubectl apply -f e2e/manifests/infrastructure/backend.yaml
    kubectl wait --timeout=2m deploy backend --for=condition=Available
    log "Test is ready to roll"
}

# We don't remove envoy gateway during teardown to avoid the reinstallation that has high cost
function teardown_suite {
    kill ${KUBECTL_FORWARDER_PID}
    log "Killed port-forwarding"
    log "Removing the sample application"
    kubectl delete -f e2e/manifests/infrastructure/backend.yaml
    log "Cleaning gateway and gatewayclass resources"
    kubectl delete -f e2e/manifests/infrastructure/gateway.yaml
    kubectl delete -f e2e/manifests/infrastructure/gatewayclass.yaml
}