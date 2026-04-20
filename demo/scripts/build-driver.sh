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

# If PUSH_IMAGE is not set, default it to true
: ${PUSH_IMAGE:="true"}

# Build the Google TPU DRA driver image
echo "Building dra-driver-google-tpu container image"
echo ${SCRIPTS_DIR}
${SCRIPTS_DIR}/build-driver-image.sh

# Go back to the root directory of this repo
cd ${CURRENT_DIR}/../..

if [ "${PUSH_IMAGE}" = "true" ]; then
    echo "Pushing container image to GCP Artifact Registry"
    make -f deployments/container/Makefile docker-push IMAGE="${DRIVER_IMAGE}"
else
    echo "Skipping image push to registry as PUSH_IMAGE is set to \"${PUSH_IMAGE}\""
fi

set +x
printf '\033[0;32m'
echo "Driver build complete: ${DRIVER_IMAGE}"
printf '\033[0m'
