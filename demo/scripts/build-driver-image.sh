#!/usr/bin/env bash

# Copyright The Kubernetes Authors.
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

# This scripts invokes `kind build image` so that the resulting
# image has a containerd with CDI support.
#
# Usage: kind-build-image.sh <tag of generated image>

# A reference to the current directory where this script is located
CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

set -ex
set -o pipefail

source "${CURRENT_DIR}/common.sh"

while [[ $# -gt 0 ]]; do
  case $1 in
    -r|--registry)
      DRIVER_IMAGE_REGISTRY="$2"
      shift # past argument
      shift # past value
      ;;
    -i|--image)
      DRIVER_IMAGE_NAME="$2"
      shift # past argument
      shift # past value
      ;;
    -t|--tag)
      DRIVER_IMAGE_TAG="$2"
      shift # past argument
      shift # past value
      ;;
    --multi-arch)
      MULTI_ARCH=true
      shift # past argument
      ;;
    -*|--*)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

echo "REGISTRY"=${REGISTRY}

# Create a temorary directory to hold all the artifacts we need for building the image
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# Go back to the root directory of this repo
cd ${CURRENT_DIR}/../..

# Set build variables
export REGISTRY=${DRIVER_IMAGE_REGISTRY}
export IMAGE=${DRIVER_IMAGE_NAME}
export TAG=${DRIVER_IMAGE_TAG}
export CONTAINER_TOOL="${CONTAINER_TOOL}"

if [[ "${MULTI_ARCH}" == "true" ]]; then
  make -f deployments/container/Makefile multi-arch-all
else
  # Regenerate the CRDs and build the container image
  make -f deployments/container/Makefile
fi
