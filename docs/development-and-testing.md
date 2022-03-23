## Development and Testing

### Running pipeline scripts locally

The script that's run in the pipeline is located in [./hack/app_sre_build_deploy.sh].
Make sure to have the following environment variables set when running it:

```
export REGISTRY_TOKEN=... # Get from quay.io user settings "Generate encrypted password"
export REGISTRY_USER=... # quay.io username
export IMG=quay.io/$REGISTRY_USER/managed-node-metadata-operator
export IMAGE_REPOSITORY=$REGISTRY_USER
export CONTAINER_ENGINE=docker # Currently this script requires docker as container engine
export QUAY_IMAGE=$IMG
export QUAY_USER=$REGISTRY_TOKEN
export QUAY_TOKEN=$REGISTRY_TOKEN
export QUAY_USER=$REGISTRY_USER
```

Additionally setting `DRY_RUN=y` will skip the push to the OLM repository and image registry.
Set `DRY_RUN=nopush` if you only want to skip the push to the OLM repository.

Before running the script, ensure the repositories quay.io/$REGISTRY_USER/managed-node-metadata-operator and quay.io/$REGISTRY_USER/managed-node-metadata-operator-registry exist.

**Tip:** You can use [direnv](https://direnv.net) and add the above block (with variables filled in) into a `.envrc` file (make sure `.envrc` is in your global git ignore as well). Upon entry to the `managed-node-metadata-operator` folder, the env vars inside the file will be loaded automatically, and unset when you leave the folder.

### Running integration tests

Integration tests use the same client library as the operator itself.
You can run them against an OpenShift cluster by running the go tests in the `int` folder:

```
go run -count=1 ./int/...
```

Make sure to exclude them when running unit tests.
They are excluded when running `make test` by default.
