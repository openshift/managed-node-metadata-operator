FIPS_ENABLED=true
TESTTARGETS=$(shell ${GOENV} go list -e ./... | egrep -v "/(vendor)/" | grep -v /int)

include boilerplate/generated-includes.mk

SHELL := /usr/bin/env bash

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update