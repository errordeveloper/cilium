# Copyright 2017-2019 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

include ../../Makefile.defs

MANAGED_ETCD_VERSION := "v2.0.7"

QUICK_INSTALL := "$(ROOT_DIR)/$(RELATIVE_DIR)/quick-install.yaml"
EXPERIMENTAL_INSTALL := "$(ROOT_DIR)/$(RELATIVE_DIR)/experimental-install.yaml"
MANAGED_ETCD_PATH := "$(ROOT_DIR)/$(RELATIVE_DIR)/cilium/charts/managed-etcd/values.yaml"
CILIUM_CHARTS := "$(ROOT_DIR)/$(RELATIVE_DIR)/cilium/"
CILIUM_VALUES := "$(CILIUM_CHARTS)/values.yaml"

VERSION_REGEX := '[0-9]\+\.[0-9]\+\.[0-9]\+.*'
LATEST_VERSION_REGEX := '[0-9]\+\.[0-9]\+\.90'
DEV_VERSION_REGEX := '[0-9]\+\.[0-9]\+\.[0-9]\+-dev'
CILIUM_CHART_REGEX := '\([vV]ersion:\) '$(VERSION_REGEX)
CILIUM_TAG_REGEX := '\(tag:\) \(v'$(VERSION_REGEX)'\|latest\)'
CILIUM_PULLPOLICY_REGEX := '\(pullPolicy:\) .*'
EXPERIMENTAL_OPTIONS := \
    --set global.hubble.enabled=true \
    --set global.hubble.listenAddress=":4244" \
    --set global.hubble.metrics.enabled="{dns,drop,tcp,flow,port-distribution,icmp,http}" \
    --set global.hubble.relay.enabled=true \
    --set global.hubble.ui.enabled=true

all: update-versions $(QUICK_INSTALL) $(EXPERIMENTAL_INSTALL)

$(QUICK_INSTALL): $(shell find cilium/ -type f)
	$(QUIET)helm template cilium --namespace=kube-system $(OPTS) > $(QUICK_INSTALL)

$(EXPERIMENTAL_INSTALL): $(shell find cilium/ -type f)
	$(QUIET)helm template cilium --namespace=kube-system $(EXPERIMENTAL_OPTIONS) > $(EXPERIMENTAL_INSTALL)

update-versions:
	$(ECHO_GEN) " -> Updating version to $(VERSION)"
	@# Update chart versions to point to the current version.
	$(QUIET)grep -lRZ -e "version:" -e "appVersion:" $(CILIUM_CHARTS) | \
		xargs -0 -l sed -i -e 's/'$(CILIUM_CHART_REGEX)'/\1 $(VERSION)/g'
	@# Fix up the cilium tag
	$(QUIET)if echo $(VERSION) | grep -q $(LATEST_VERSION_REGEX); then				\
			sed -i 's/'$(CILIUM_TAG_REGEX)'/\1 latest/' $(CILIUM_VALUES);			\
			sed -i 's/'$(CILIUM_PULLPOLICY_REGEX)'/\1 Always/' $(CILIUM_VALUES);		\
		elif echo $(VERSION) | grep -q $(DEV_VERSION_REGEX); then				\
			DEV_BRANCH=$$(echo $(VERSION) | sed 's/-dev//')					\
			sed -i 's/'$(CILIUM_TAG_REGEX)'/\1 $(DEV_BRANCH)/' $(CILIUM_VALUES);		\
			sed -i 's/'$(CILIUM_PULLPOLICY_REGEX)'/\1 Always/' $(CILIUM_VALUES);		\
		else											\
			sed -i 's/'$(CILIUM_TAG_REGEX)'/\1 v$(VERSION)/' $(CILIUM_VALUES);		\
			sed -i 's/'$(CILIUM_PULLPOLICY_REGEX)'/\1 IfNotPresent/' $(CILIUM_VALUES);	\
		fi
	@# Fix up the managed etcd version, as that has its own scheme
	$(QUIET)sed -i 's/'$(VERSION)'/'$(MANAGED_ETCD_VERSION)'/' $(MANAGED_ETCD_PATH)

clean:
	$(RM) $(QUICK_INSTALL) $(EXPERIMENTAL_INSTALL)

.phony: all clean update-versions
