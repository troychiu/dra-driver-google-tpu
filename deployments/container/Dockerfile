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

ARG GOLANG_VERSION=undefined
FROM google-go.pkg.dev/golang:1.26.0 as build

WORKDIR /build
COPY . .

RUN mkdir /artifacts
RUN make PREFIX=/artifacts cmds

FROM gcr.io/distroless/base:latest

LABEL io.k8s.display-name="TPU resource Driver for Dynamic Resource Allocation (DRA)"
LABEL name="TPU Resource Driver for Dynamic Resource Allocation (DRA)"
LABEL vendor="google.com"
LABEL summary="TPU DRA resource driver for Kubernetes"
LABEL description="See summary"

COPY --from=build /artifacts/tpu-dra-kubeletplugin /usr/bin/tpu-dra-kubeletplugin
