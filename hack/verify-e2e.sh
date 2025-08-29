#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# TODO: make this configurable
export PROVIDER="ingress-nginx"
export I2G="${I2G:-$(pwd)/ingress2gateway}"
export TESTS_DIR="${TESTS_DIR:-$(pwd)/e2e/tests}"

  
function pre_check() {
    if ! ${I2G} &>/dev/null; then 
        echo "Error executing ingress2gateway, please be sure the variable I2G is pointing to the right location"
        exit 1
    fi
    if ! command -v bats &> /dev/null; then
        echo "BATS needs to be installed. Please check https://bats-core.readthedocs.io/en/stable/installation.html"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        echo "cURL needs to be installed to execute the tests"
        exit 1
    fi
    
    if ! kubectl version; then
        echo "Error executing kubectl. Please be sure you have a Kubernetes cluster running before executing the test"
        exit 1
    fi
}

function run_tests() {
    bats "${TESTS_DIR}"
}

pre_check
run_tests