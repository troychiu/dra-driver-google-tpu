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

##### Global variables #####
CONTAINER_TOOL ?= docker
MKDIR ?= mkdir
DOCKERFILE := $(CURDIR)/deployments/container/Dockerfile
ARCHITECTURES=amd64 arm64

include $(CURDIR)/common.mk

##### Public rules #####
.PHONY: docker-build

# Use a generic build target to build the relevant images
docker-build:
	DOCKER_BUILDKIT=1 \
		$(CONTAINER_TOOL) build --pull \
		--tag $(REGISTRY)/$(IMAGE):$(TAG) \
		--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
		--build-arg VERSION="$(VERSION)" \
		-f $(DOCKERFILE) \
		$(CURDIR)

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMAGE}

container-multi-arch:
	@docker buildx inspect img-builder > /dev/null 2>&1 \
		|| docker buildx create --name img-builder --use

	@for arch in $(ARCHITECTURES); do \
		echo ">> Building for architecture: $${arch}"; \
			docker buildx build \
			--platform linux/$${arch} \
			--build-arg ARCH=$${arch} \
			--output=type=docker \
			-t ${REGISTRY}/$(IMAGE)-$${arch}:$(TAG) \
			--pull \
			-f $(DOCKERFILE) \
			$(CURDIR); \
	done

push-all-individual:
	for arch in $(ARCHITECTURES); do \
		docker push ${REGISTRY}/${IMAGE}-$$arch:${TAG}; \
	done

build-multi-arch:
	docker manifest create --amend ${REGISTRY}/${IMAGE}:${TAG} $(shell echo $(ARCHITECTURES) | sed -e "s~[^ ]*~$(REGISTRY)/$(IMAGE)\-&:$(TAG)~g")
	@for arch in $(ARCHITECTURES); do \
		docker manifest annotate --arch $${arch} \
			${REGISTRY}/${IMAGE}:${TAG} $(REGISTRY)/$(IMAGE)-$${arch}:${TAG}; \
	done

push-multi-arch:
	docker manifest push ${REGISTRY}/$(IMAGE):$(TAG)

multi-arch-all: container-multi-arch push-all-individual build-multi-arch push-multi-arch