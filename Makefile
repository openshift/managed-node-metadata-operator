export KONFLUX_BUILDS=true
FIPS_ENABLED=true
TESTTARGETS=$(shell ${GOENV} go list -e ./... | egrep -v "/(vendor)/" | grep -v /int)
PKO_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME)-pko:$(CURRENT_COMMIT)

include boilerplate/generated-includes.mk

SHELL := /usr/bin/env bash

.PHONY: pko-build
pko-build:
	# FIXME: Hack for now. App-interface CICD does not have a way to retrieve an image if it doesn't use either <channel>-<commit> tag or just <commit>.
	# Our images use v<...>-<commit> which isn't well supported to be used by app-interface.
	sed -i 's#{{ .config.image }}#$(OPERATOR_IMAGE_URI)#' ./deploy_pko/Deployment*.yaml
	$(CONTAINER_ENGINE) build -f ./build/Dockerfile.pko -t $(PKO_IMAGE) ./deploy_pko
	# git checkout ./deploy/pko/

.PHONY: pko-build-push
pko-build-push: pko-build container-engine-login
	$(CONTAINER_ENGINE) push $(PKO_IMAGE)

.PHONY: pko-validation
pko-validation:
	kubectl package tree --cluster ./deploy/pko
	kubectl package validate ./deploy/pko

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update
