FROM quay.io/openshift/origin-operator-registry@sha256:a0e3fa5b83fbc66419dfb7efad3990e4fc747b3e1bb0b36c2c063cd54284fbf2

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=managed-node-metadata-operator
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.2.0
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v2
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1


ARG BUNDLE_DIR=packagemanifests
COPY $BUNDLE_DIR manifests
RUN ls manifests
RUN initializer --permissive

CMD ["registry-server", "-t", "/tmp/terminate.log"]
