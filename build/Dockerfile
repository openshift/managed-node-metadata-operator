FROM quay.io/redhat-services-prod/openshift/boilerplate:latest AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the whole tree: `make go-build` runs the boilerplate `ensure-fips`
# generator first, which needs boilerplate/ and the git metadata.
COPY . .

# Build via boilerplate so the image gets the FIPS wiring the Makefile asks
# for (FIPS_ENABLED=true): the generated fips.go assertion, GOFIPS140, the
# no_openssl build tag and the dynamic-linking check.
RUN make go-build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.9-1795198179

ENV USER_UID=1001 \
    USER_NAME=managed-node-metadata-operator

WORKDIR /
COPY --from=builder /workspace/build/_output/bin/managed-node-metadata-operator /manager
USER ${USER_UID}

ENTRYPOINT ["/manager"]
