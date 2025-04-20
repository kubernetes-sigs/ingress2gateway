#!/bin/bash

# Copyright 2025 The Kubernetes Authors.
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

# Verifies that the auto-generated documentation sections in README.md are up-to-date.

# Exit immediately if a command exits with a non-zero status.
set -e

# Pipe failures should exit the script
set -o pipefail

echo "+++ Verifying README.md documentation..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
README_FILE="${REPO_ROOT}/README.md"
UPDATE_SCRIPT="${SCRIPT_DIR}/generate-cli-docs.sh" # Path to your update script

# Ensure the script runs from the repository root for consistent paths
cd "${REPO_ROOT}"

# 1. Ensure the update script exists and is executable
if [ ! -f "$UPDATE_SCRIPT" ]; then
    echo "ERROR: Documentation update script not found at $UPDATE_SCRIPT" >&2
    exit 1
fi
if [ ! -x "$UPDATE_SCRIPT" ]; then
    echo "ERROR: Documentation update script is not executable: $UPDATE_SCRIPT" >&2
    exit 1
fi

# 2. Run the update script to potentially regenerate the docs in place.
#    We capture output to avoid polluting CI logs unless the script fails.
echo "--- Running documentation update script to ensure consistency..."
update_output=$(mktemp)
if ! "$UPDATE_SCRIPT" > "$update_output" 2>&1; then
    echo "ERROR: Documentation update script '${UPDATE_SCRIPT}' failed. Output:" >&2
    cat "$update_output" >&2
    rm "$update_output"
    exit 1
fi
rm "$update_output"
echo "--- Documentation update script finished."

# 3. Check for differences in README.md using git diff.
#    'git diff --exit-code' returns 0 if there are no differences, non-zero otherwise.
echo "--- Checking for changes in ${README_FILE}..."
if git diff --quiet HEAD -- "${README_FILE}"; then
    echo "+++ SUCCESS: ${README_FILE} documentation is up-to-date."
    exit 0
else
    echo "--- ERROR: ${README_FILE} documentation is out-of-date." >&2
    echo "--- Please run './hack/generate-cli-docs.sh' (or 'make docs')" >&2
    echo "--- and commit the changes to ${README_FILE}." >&2
    exit 1
fi