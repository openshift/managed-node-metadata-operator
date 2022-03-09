FROM quay.io/operator-framework/operator-sdk:v1.2.0 AS operator-sdk

# We need git to clone our repo
RUN microdnf install -y git gcc golang-1.16.12 make which

# install kustomize
RUN git clone https://github.com/kubernetes-sigs/kustomize.git
RUN cd kustomize && \
      git checkout kustomize/v3.8.8 && \
    cd kustomize && \
    go install .
RUN ~/go/bin/kustomize version
# Clean up after install
RUN rm -rf /.cache

# Set workdir so we have a known location to copy files from
RUN mkdir /pipeline
WORKDIR /pipeline
COPY ./ managed-node-metadata-operator

# Make all the things
ENTRYPOINT ["./managed-node-metadata-operator/hack/bundler.sh"]
