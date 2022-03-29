#!/bin/bash

set -exv

# prefix var with _ so we don't clober the var used during the Make build
# it probably doesn't matter but we can change it later.
_OPERATOR_NAME="managed-node-metadata-operator"

BRANCH_CHANNEL="$1"
QUAY_IMAGE="$2"

GIT_HASH=$(git rev-parse --short=7 HEAD)
GIT_COMMIT_COUNT=$(git rev-list $(git rev-list --max-parents=0 HEAD)..HEAD --count)

# clone bundle repo
SAAS_OPERATOR_DIR="saas-managed-node-metadata-operator-bundle"
BUNDLE_DIR="$SAAS_OPERATOR_DIR/managed-node-metadata-operator/"

rm -rf "$SAAS_OPERATOR_DIR"

SAAS_BUNDLE_URI=${SAAS_BUNDLE_URI:-https://app:${APP_SRE_BOT_PUSH_TOKEN}@gitlab.cee.redhat.com/service/saas-managed-node-metadata-operator-bundle.git}
git clone \
    --branch "$BRANCH_CHANNEL" \
    "$SAAS_BUNDLE_URI" \
    "$SAAS_OPERATOR_DIR"

# remove any versions more recent than deployed hash
REMOVED_VERSIONS=""
if [[ "$BRANCH_CHANNEL" == "production" ]]; then
    SAAS_FILE_EXISTS=$(curl -s "https://gitlab.cee.redhat.com/service/app-interface/raw/master/data/services/osd-operators/cicd/saas/saas-${_OPERATOR_NAME}.yaml" -o saasfile.yaml ; echo $?)

    if [[ "$SAAS_FILE_EXISTS" != "0" ]]; then
        echo "Can't get SAAS file, assuming the operator is not deployed yet, exiting"
        exit 0
    fi
    DEPLOYED_HASH=$(docker run --rm -i quay.io/app-sre/yq:3.4.1 yq r - "resourceTemplates[*].targets(namespace.\$ref==/services/osd-operators/namespaces/hivep01ue1/cluster-scope.yml).ref" < saasfile.yaml)

    # Ensure that our query for the current deployed hash worked
    # Validate that our DEPLOYED_HASH var isn't empty.
    # Although we have `set -e` defined the docker container isn't returning
    # an error and allowing the script to continue
    echo "Current deployed production HASH: $DEPLOYED_HASH"

    if [[ ! "${DEPLOYED_HASH}" =~ [0-9a-f]{40} ]]; then
        echo "Can't discover current production deployed HASH, assuming the operator was not yet promoted to production."
    else
        delete=false
        # Sort based on commit number
        for version in $(ls $BUNDLE_DIR | sort -t . -k 3 -g); do
            # skip if not directory
            [ -d "$BUNDLE_DIR/$version" ] || continue

            if [[ "$delete" == false ]]; then
                short_hash=$(echo "$version" | cut -d- -f2)

                if [[ "$DEPLOYED_HASH" == "${short_hash}"* ]]; then
                    delete=true
                fi
            else
                rm -rf "${BUNDLE_DIR:?BUNDLE_DIR var not set}/$version"
                REMOVED_VERSIONS="$version $REMOVED_VERSIONS"
            fi
        done
    fi
fi

# generate bundle
PREV_VERSION=$(ls "$BUNDLE_DIR" | sort -t . -k 3 -g | tail -n 1)

# build the registry image
IMAGE_DIGEST=$(skopeo inspect docker://$QUAY_IMAGE:$GIT_HASH | jq -r .Digest)
if [[ -z "$IMAGE_DIGEST" ]]; then
    echo "Couldn't discover IMAGE_DIGEST for docker://${QUAY_IMAGE}:${GIT_HASH}!"
    exit 1
fi
REPO_DIGEST=${QUAY_IMAGE}@${IMAGE_DIGEST}

# Build an image locally that has all tools we need
#
# TODO: This section is done as in the pipeline stuff is running on the Jenkins slave
#       that doesn't have `operator-sdk >=1` and `kustomize`
#       so we build a container with those tools and run all operations inside
#
# what this section does in general is `make packagemanifests`
docker rm managed-node-metadata-operator-pipeline || true
docker build -f hack/pipeline.dockerfile -t pipelinebuilder:latest .
docker run  \
-e "CHANNELS=$BRANCH_CHANNEL" \
-e "IMG=$REPO_DIGEST" \
-e "PREV_VERSION=$PREV_VERSION" \
-e "BUNDLE_DIR=$BUNDLE_DIR" \
--name managed-node-metadata-operator-pipeline \
pipelinebuilder:latest
docker cp "managed-node-metadata-operator-pipeline:/pipeline/managed-node-metadata-operator/$BUNDLE_DIR" packagemanifests
docker rm managed-node-metadata-operator-pipeline
rsync -a packagemanifests/* $BUNDLE_DIR/
rm -rf packagemanifests

BUNDLE_DIR=$BUNDLE_DIR BUNDLE_IMG="${REGISTRY_IMAGE}:${BRANCH_CHANNEL}-latest" PREV_VERSION="$PREV_VERSION" make packagemanifests-build

pushd $SAAS_OPERATOR_DIR

git add .

MESSAGE="add version $GIT_COMMIT_COUNT-$GIT_HASH

replaces $PREV_VERSION
removed versions: $REMOVED_VERSIONS"

git commit -m "$MESSAGE"
popd

NEW_VERSION=$(ls "$BUNDLE_DIR" | sort -t . -k 3 -g | tail -n 1)

if [ "$NEW_VERSION" == "$PREV_VERSION" ]; then
    # stopping script as that version was already built, so no need to rebuild it
    exit 0
fi

if [[ "$DRY_RUN" == "y" ]]; then
  exit 0
fi


if [[ "$DRY_RUN" != "nopush" ]]; then
    pushd $SAAS_OPERATOR_DIR
    git push origin "$BRANCH_CHANNEL"
    popd
fi

# push image
skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
    "docker-daemon:${REGISTRY_IMAGE}:${BRANCH_CHANNEL}-latest" \
    "docker://${REGISTRY_IMAGE}:${BRANCH_CHANNEL}-latest"

skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
    "docker-daemon:${REGISTRY_IMAGE}:${BRANCH_CHANNEL}-latest" \
    "docker://${REGISTRY_IMAGE}:${BRANCH_CHANNEL}-${GIT_HASH}"
