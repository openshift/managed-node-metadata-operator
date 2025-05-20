FIPS_ENABLED=true
TESTTARGETS=$(shell ${GOENV} go list -e ./... | egrep -v "/(vendor)/" | grep -v /int)
PKO_PRODUCTION_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME)-pko:production-$(CURRENT_COMMIT)
PKO_STAGING_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME)-pko:staging-$(CURRENT_COMMIT)

include boilerplate/generated-includes.mk

SHELL := /usr/bin/env bash

.PHONY: pko-build
pko-build:
	# FIXME: Hack for now. App-interface CICD does not have a way to retrieve an image if it doesn't use <channel>-<commit> tag or just <commit>.
	# Our images use v<...>-<commit> which isn't well supported to be used by app-interface.
	sed -i 's#{{ .config.image }}#$(OPERATOR_IMAGE_URI)#' ./deploy/pko/Deployment*.yaml
	$(CONTAINER_ENGINE) build -f ./build/Dockerfile.pko -t $(PKO_STAGING_IMAGE) ./deploy/pko
	$(CONTAINER_ENGINE) build -f ./build/Dockerfile.pko -t $(PKO_PRODUCTION_IMAGE) ./deploy/pko
	git checkout ./deploy/pko/

pko-build-push: pko-build container-engine-login
	$(CONTAINER_ENGINE) push $(PKO_STAGING_IMAGE)
	$(CONTAINER_ENGINE) push $(PKO_PRODUCTION_IMAGE)

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update
