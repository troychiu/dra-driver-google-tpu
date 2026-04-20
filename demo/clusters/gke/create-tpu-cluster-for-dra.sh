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


PROJECT_NAME=$(gcloud config get-value project)
if [[ -z ${PROJECT_NAME} ]]; then
	echo "Project name could not be determined"
	echo "Please run 'gcloud config set project'"
	exit 1
fi

CLUSTER_NAME="$USER-tpu-cluster"
NODE_POOL_NAME="tpu-nodepool"
# Make variables configurable from cmd.
export NODE_VERSION="${NODE_VERSION:-1.34}"
export REGION="${REGION:-us-central2-b}"
export MACHINE_TYPE="${MACHINE_TYPE:-ct4p-hightpu-4t}"

echo "Project: $PROJECT_NAME"
echo "Cluster: $CLUSTER_NAME"
echo "Node Pool: $NODE_POOL_NAME"
echo "Version: $NODE_VERSION"
echo "Machine: $MACHINE_TYPE"
echo "Region: $REGION"

read -p "Proceed with these settings? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
  exit 1
fi

# Disable the tpu-device-plugin by adding node label to nodepool.
# The label only works on or after GKE version: 1.34.1-gke.1127000
LABELS="cloud.google.com/gke-tpu-dra-driver=true"

CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

# Create the cluster
gcloud container clusters create "${CLUSTER_NAME}" \
  --quiet \
  --release-channel=rapid \
  --no-enable-autorepair \
  --enable-autoupgrade \
  --region "${REGION}" \
  --num-nodes "1" \
  --cluster-version "${NODE_VERSION}" \
  --node-version "${NODE_VERSION}" \
  --workload-pool="${PROJECT_NAME}.svc.id.goog"

# autorepair enabled since cl/868430546 is now merged.
gcloud container node-pools create $NODE_POOL_NAME \
  --cluster $CLUSTER_NAME \
  --num-nodes 1 \
  --region "${REGION}" \
  --node-version "${NODE_VERSION}" \
  --node-labels $LABELS \
  --machine-type="${MACHINE_TYPE}" \
  --enable-autoupgrade \
  --enable-autorepair \
  --workload-metadata=GKE_METADATA

# Wait for the node pool to be ready
while true; do
  NODE_POOL_STATUS=$(gcloud container node-pools describe $NODE_POOL_NAME \
    --cluster $CLUSTER_NAME --region "${REGION}" --format="value(status)")
  if [[ $NODE_POOL_STATUS == "RUNNING" ]]; then
    break
  fi
  echo "Waiting for the node pool to be ready..."
  sleep 10
done

gcloud container clusters get-credentials "${CLUSTER_NAME}" \
  --region "${REGION}" \
  --project "${PROJECT_NAME}"

## Create the dra-driver-google-tpu namespace
kubectl create namespace dra-driver-google-tpu
