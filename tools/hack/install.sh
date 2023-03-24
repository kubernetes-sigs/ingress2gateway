#!/usr/bin/env bash

# Copyright © 2023 Kubernetes Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

# 	http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

: "${BINARY_NAME:="i2gw"}"
: "${I2GW_INSTALL_DIR:="/usr/local/bin"}"

export VERSION

HAS_CURL="$(type "curl" &> /dev/null && echo true || echo false)"
HAS_WGET="$(type "wget" &> /dev/null && echo true || echo false)"
HAS_GIT="$(type "git" &> /dev/null && echo true || echo false)"

GH_REPO="Xunzhuo/ingress2gateway"

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="arm";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
  esac
}

# initOS discovers the operating system for this system.
initOS() {
  OS="$(uname|tr '[:upper:]' '[:lower:]')"

  case "$OS" in
    # Minimalist GNU for Windows
    mingw*|cygwin*) OS='windows';;
  esac
}

# runs the given command as root (detects if we are root already)
runAsRoot() {
  if [ $EUID -ne 0 ]; then
    sudo "${@}"
  else
    "${@}"
  fi
}

# verifySupported checks that the os/arch combination is supported for
# binary builds, as well whether or not necessary tools are present.
verifySupported() {
  local supported="darwin-amd64\ndarwin-arm64\nlinux-amd64\nlinux-arm64\n"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "No prebuilt binary for ${OS}-${ARCH}."
    echo "To build from source, go to https://github.com/${GH_REPO}"
    exit 1
  fi

  if [ "${HAS_CURL}" != "true" ] && [ "${HAS_WGET}" != "true" ]; then
    echo "Either curl or wget is required"
    exit 1
  fi

  if [ "${HAS_GIT}" != "true" ]; then
    echo "[WARNING] Could not find git. It is required for plugin installation."
  fi
}

# checkDesiredVersion checks if the desired version is available.
checkDesiredVersion() {
  if [ "$VERSION" == "" ]; then
    # Get tag from release URL
    local latest_release_url="https://github.com/${GH_REPO}/releases"
    if [ "${HAS_CURL}" == "true" ]; then
      VERSION=$(curl -Ls $latest_release_url | grep 'href="/${GH_REPO}/releases/tag/v[0-9]*.[0-9]*.[0-9]*\"' | sed -E 's/.*\/Xunzhuo\/ingress2gateway\/releases\/tag\/(v[0-9\.]+)".*/\1/g' | head -1)
    elif [ "${HAS_WGET}" == "true" ]; then
      VERSION=$(wget $latest_release_url -O - 2>&1 | grep 'href="/${GH_REPO}/releases/tag/v[0-9]*.[0-9]*.[0-9]*\"' | sed -E 's/.*\/Xunzhuo\/ingress2gateway\/releases\/tag\/(v[0-9\.]+)".*/\1/g' | head -1)
    fi
      if [ "$VERSION" == "" ]; then
        VERSION="latest"
      fi
  fi
}

# checkI2GWInstalledVersion checks which version of i2gw is installed and
# if it needs to be changed.
checkI2GWInstalledVersion() {
  if [[ -f "${I2GW_INSTALL_DIR}/${BINARY_NAME}" ]]; then
    version=$("${I2GW_INSTALL_DIR}/${BINARY_NAME}" version | grep -Eo "v[0-9]+\.[0-9]+.*" )
    if [[ "$version" == "$VERSION" ]]; then
      echo "i2gw ${version} is already ${VERSION:-latest}"
      return 0
    else
      echo "i2gw ${VERSION} is available. Changing from version ${version}."
      return 1
    fi
  else
    return 1
  fi
}

# downloadFile downloads the latest binary package
# for that binary.
downloadFile() {
  I2GW_DIST="i2gw_${VERSION}_${OS}_${ARCH}.tar.gz"
  DOWNLOAD_URL="https://github.com/${GH_REPO}/releases/download/$VERSION/$I2GW_DIST"
  I2GW_TMP_ROOT="$(mktemp -dt i2gw-installer-XXXXXX)"
  I2GW_TMP_FILE="$I2GW_TMP_ROOT/$I2GW_DIST"
  echo "Downloading $DOWNLOAD_URL"
  if [ "${HAS_CURL}" == "true" ]; then
    curl -SsL "$DOWNLOAD_URL" -o "$I2GW_TMP_FILE"
  elif [ "${HAS_WGET}" == "true" ]; then
    wget -q -O "$I2GW_TMP_FILE" "$DOWNLOAD_URL"
  fi
}

# installFile installs the i2gw binary.
installFile() {
  I2GW_TMP="$I2GW_TMP_ROOT/$BINARY_NAME"
  mkdir -p "$I2GW_TMP"
  tar xf "$I2GW_TMP_FILE" -C "$I2GW_TMP"
  I2GW_TMP_BIN="$I2GW_TMP/_output/$OS/$ARCH/i2gw"
  echo "Preparing to install $BINARY_NAME into ${I2GW_INSTALL_DIR}"
  runAsRoot cp "$I2GW_TMP_BIN" "$I2GW_INSTALL_DIR/$BINARY_NAME"
  echo "$BINARY_NAME installed into $I2GW_INSTALL_DIR/$BINARY_NAME"
}

# fail_trap is executed if an error occurs.
fail_trap() {
  result=$?
  if [ "$result" != "0" ]; then
    if [[ -n "$INPUT_ARGUMENTS" ]]; then
      echo "Failed to install $BINARY_NAME with the arguments provided: $INPUT_ARGUMENTS"
    else
      echo "Failed to install $BINARY_NAME"
    fi
    echo -e "\tFor support, go to https://github.com/${GH_REPO}."
  fi
  cleanup
  exit $result
}

# testVersion tests the installed client to make sure it is working.
testVersion() {
  set +e
  if ! [ "$(command -v $BINARY_NAME)" ]; then
    echo "$BINARY_NAME not found. Is $I2GW_INSTALL_DIR on your PATH?"
    exit 1
  fi
  set -e
}

# cleanup temporary files.
cleanup() {
  if [[ -d "${I2GW_TMP_ROOT:-}" ]]; then
    rm -rf "$I2GW_TMP_ROOT"
  fi
}

# Execution

#Stop execution on any error
trap "fail_trap" EXIT
set -e

initArch
initOS
verifySupported
checkDesiredVersion
if ! checkI2GWInstalledVersion; then
  downloadFile
  installFile
fi
testVersion
cleanup
