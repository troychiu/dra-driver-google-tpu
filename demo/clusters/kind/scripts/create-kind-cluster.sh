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

# A reference to the current directory where this script is located
CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

set -ex
set -o pipefail

source "${CURRENT_DIR}/common.sh"

${KIND} create cluster \
	--name "${KIND_CLUSTER_NAME}" \
	--image "${KIND_IMAGE}" \
	--config "${KIND_CLUSTER_CONFIG_PATH}" \
	--wait 2m

# Create fake devices on the worker node
echo "Creating fake devices on worker nodes"

# Define the DaemonSet to create fake devices, similar to GKE script
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fake-tpu-devices
  namespace: kube-system
  labels:
    app: fake-tpu-devices
spec:
  selector:
    matchLabels:
      app: fake-tpu-devices
  template:
    metadata:
      labels:
        app: fake-tpu-devices
    spec:
      hostPID: true
      initContainers:
      - name: fake-devices
        image: debian:bookworm-slim
        command:
        - /bin/sh
        - -c
        - |
          # Use nsenter to create devices on the host (the kind node container)
          nsenter -t 1 -m -u -i -n -- bash -c '
            echo "Creating fake TPU devices";
            for i in {0..3}; do
              if [ ! -b /dev/accel\$i ]; then
                mknod /dev/accel\$i b 100 \$i
                chmod 666 /dev/accel\$i
              fi
            done
          '
        securityContext:
          privileged: true
      containers:
      - name: pause
        image: registry.k8s.io/pause:3.10
EOF

# Wait for the DaemonSet to be ready
kubectl rollout status daemonset/fake-tpu-devices -n kube-system --timeout=60s
