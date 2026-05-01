#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
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

# A reference to the current directory where this script is located
CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

set -ex
set -o pipefail

source "${CURRENT_DIR}/common.sh"

# Variables REGISTRY, IMAGE, and TAG are inherited from environment or set as defaults in common.sh
# DRIVER_NAME is also available from common.sh

echo "REGISTRY=${REGISTRY}"
echo "DRIVER_NAME=${DRIVER_NAME}"

# Go back to the root directory of this repo
cd "${CURRENT_DIR}/../.."

DIST_DIR="dist"
mkdir -p "${DIST_DIR}"

# Clean up any old packages for this chart to avoid wildcard issues
rm -f "${DIST_DIR}/${DRIVER_NAME}-*.tgz"

# Derive helm version from TAG (remove leading 'v' if present)
HELM_VERSION="${CHART_VERSION:-${TAG#v}}"
echo "HELM_VERSION=${HELM_VERSION}"

HELM="${HELM:-helm}"

# Package the helm chart with the specified version
${HELM} package deployments/helm/${DRIVER_NAME} --version "${HELM_VERSION}" --destination "${DIST_DIR}"

# Push to OCI registry using the exact filename
${HELM} push "${DIST_DIR}/${DRIVER_NAME}-${HELM_VERSION}.tgz" "oci://${REGISTRY}/charts"
