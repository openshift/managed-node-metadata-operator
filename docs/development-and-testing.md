# Development and Testing
## Running pipeline scripts locally

The script that's run in the pipeline is located [here](../hack/app_sre_build_deploy.sh), this script is used to build and deploy the operator image to the app-sre repository. We'll also detail how to build and push an image to your own quay repository.

### Setting environment variables

Make sure to have the following environment variables set:
```
export REGISTRY_TOKEN=... # Get from quay.io user settings "Generate encrypted password"
export REGISTRY_USER=... # quay.io username
export IMG=quay.io/$REGISTRY_USER/managed-node-metadata-operator
export IMAGE_REPOSITORY=$REGISTRY_USER
export CONTAINER_ENGINE=docker # Currently this script requires docker as container engine
export QUAY_IMAGE=$IMG
export QUAY_TOKEN=$REGISTRY_TOKEN
export QUAY_USER=$REGISTRY_USER
```

Additionally setting `DRY_RUN=y` will skip the push to the OLM repository and image registry.
Set `DRY_RUN=nopush` if you only want to skip the push to the OLM repository.

**Tip:** You can use [direnv](https://direnv.net) and add the above block (with variables filled in) into a `.envrc` file (make sure `.envrc` is in your global git ignore as well). Upon entry to the `managed-node-metadata-operator` folder, the env vars inside the file will be loaded automatically, and unset when you leave the folder.

### Creating your quay repository

Before running the script, ensure the repositories are created on quay.io
- quay.io/$REGISTRY_USER/managed-node-metadata-operator
- quay.io/$REGISTRY_USER/managed-node-metadata-operator-registry

### Build and push your images

To build and push your image for app-sre, use [./hack/app_sre_build_deploy.sh].
To build and push your image for your own quay repository, run the following:
```bash
make docker-build docker-push
```
## Running integration tests

### Deploying your image

Integration tests use the same client library as the operator itself. Before running the integration tests, ensure you have set your environment variables and have the operator image available in your personal quay repository, as described in the previous sections.

To deploy your image to your chosen cluster, run the following:
```bash
make deploy
```
You can follow the operator logs with the following command:
```bash
oc logs -f -n openshift-managed-node-metadata-operator $(oc get pods -n openshift-managed-node-metadata-operator -ojson | jq -r '.items[0].metadata.name')
```

### Running tests

You can run them against an OpenShift cluster by running the go tests in the `int` folder:
```
go test -count=1 ./int/...
```
Make sure to exclude them when running unit tests.
They are excluded when running `make test` by default.
