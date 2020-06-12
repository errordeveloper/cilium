# Copyright 2020 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

REGISTRIES ?= docker.io/cilium
# quay.io is not enabled, see https://github.com/cilium/image-tools/issues/11
# REGISTRIES ?= docker.io/cilium quay.io/cilium

PUSH ?= false

OUTPUT := "type=docker"
ifeq ($(PUSH),true)
OUTPUT := "type=registry,push=true"
endif

all-images: lint runtime-image

lint:
	scripts/lint.sh

.buildx_builder:
	# see https://github.com/docker/buildx/issues/308
	mkdir -p ../.buildx
	docker buildx create --platform linux/amd64,linux/arm64 --buildkitd-flags '--debug' > $@

runtime-image: .buildx_builder
	scripts/build-image.sh cilium-runtime images/runtime linux/amd64,linux/arm64 $(OUTPUT) "$$(cat .buildx_builder)" $(REGISTRIES)

builder-image: .buildx_builder
	scripts/build-image.sh cilium-builder images/builder linux/amd64 $(OUTPUT) "$$(cat .buildx_builder)" $(REGISTRIES)

cilium-image: .buildx_builder
	cd .. && git archive @ | docker buildx build --tag $(firstword $(REGISTRIES))/cilium:$(shell scripts/make-image-tag.sh) --platform linux/amd64,linux/arm64 --builder "$$(cat images/.buildx_builder)" --output $(OUTPUT) --file images/cilium/Dockerfile -

operator-image: .buildx_builder
	cd .. && git archive @ | docker buildx build --tag $(firstword $(REGISTRIES))/operator:$(shell scripts/make-image-tag.sh) --platform linux/amd64,linux/arm64 --builder "$$(cat images/.buildx_builder)" --output $(OUTPUT) --file images/operator/Dockerfile -

hubble-relay-image: .buildx_builder
	cd .. && git archive @ | docker buildx build --tag $(firstword $(REGISTRIES))/hubble-relay:$(shell scripts/make-image-tag.sh) --platform linux/amd64,linux/arm64 --builder "$$(cat images/.buildx_builder)" --output $(OUTPUT) --file images/hubble-relay/Dockerfile -
