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

# Runs the 'generate-docs' command from the ingress2gateway binary
# and updates the README.md file with the generated documentation.

# Exit immediately if a command exits with a non-zero status.
set -e
# Treat unset variables as an error when substituting.
set -u
# Pipe failures should exit the script
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
README_FILE="${REPO_ROOT}/README.md"
BINARY_PATH="${REPO_ROOT}/ingress2gateway"
# This marker in the README file indicates where to insert the generated documentation.
MARKER_START="<!--- BEGIN GENERATED DOCS -->"
MARKER_END="<!--- END GENERATED DOCS -->"
# ./ingress2gateway generate-docs is the command to generate these docs.
DOCS_COMMAND="${BINARY_PATH} generate-docs"
TMP_DOCS_FILE=$(mktemp) # Create a temporary file for docs output

# Ensure the script runs from the repo root
cd "${REPO_ROOT}"

# Cleanup function to remove temporary file on exit
cleanup() {
  rm -f "$TMP_DOCS_FILE"
}
trap cleanup EXIT

# Generate documentation
echo ">>> Generating documentation using '${DOCS_COMMAND}'..."
if ! ${DOCS_COMMAND} > "$TMP_DOCS_FILE"; then
    echo "ERROR: Failed to generate documentation." >&2
    # Optional: Print captured output for debugging
    # cat "$TMP_DOCS_FILE" >&2
    exit 1
fi
echo ">>> Documentation generated successfully."

# Check if markers exist in README
if ! grep -qF "$MARKER_START" "$README_FILE" || ! grep -qF "$MARKER_END" "$README_FILE"; then
    echo "ERROR: Start ('$MARKER_START') or end ('$MARKER_END') marker not found in $README_FILE." >&2
    echo "Please add the markers around the section to be updated in $README_FILE." >&2
    exit 1
fi

echo ">>> Updating $README_FILE..."

# Use awk to replace content between markers
awk -v start_marker="$MARKER_START" \
    -v end_marker="$MARKER_END" \
    -v docs_file="$TMP_DOCS_FILE" '
BEGIN { printing=1; found_start=0 }
# Match the start marker line
$0 == start_marker {
    print; # Print the start marker line itself
    # Print the content from the generated docs file
    while ((getline line < docs_file) > 0) {
        print line
    }
    close(docs_file);
    printing=0; # Stop printing original lines between markers
    found_start=1;
    next; # Move to next line of README input
}
# Match the end marker line
$0 == end_marker {
    if (found_start == 0) {
        print "Error: End marker found before start marker." > "/dev/stderr"
        exit 1
    }
    print; # Print the end marker line itself
    printing=1; # Resume printing original lines
    next; # Move to next line of README input
}
# Print lines only if printing is enabled (outside the markers)
{ if (printing) print }
' "$README_FILE" > "$README_FILE.tmp" && mv "$README_FILE.tmp" "$README_FILE"
# Check awk exit status (though awk script has basic checks)
if [ $? -ne 0 ]; then
    echo "ERROR: awk command failed while updating $README_FILE." >&2
    exit 1
fi

echo ">>> $README_FILE updated successfully."
exit 0