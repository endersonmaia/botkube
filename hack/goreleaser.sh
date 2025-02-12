#!/bin/bash

set -o errexit
set -o pipefail

IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-kubeshop/botkube}"
TEST_IMAGE_REPOSITORY="${TEST_IMAGE_REPOSITORY:-kubeshop/botkube-test}"
IMAGE_SAVE_LOAD_DIR="${IMAGE_SAVE_LOAD_DIR:-/tmp/botkube-images}"

prepare() {
  export DOCKER_CLI_EXPERIMENTAL="enabled"
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
}

release_snapshot() {
  prepare
  export GORELEASER_CURRENT_TAG=v9.99.9-dev
  goreleaser release --rm-dist --snapshot --skip-publish
  # Push images
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-amd64
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-arm64
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-armv7
  docker push ${IMAGE_REGISTRY}/${TEST_IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}
  # Create manifest
  docker manifest create ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG} \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-amd64 \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-arm64 \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-armv7
  docker manifest push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}
}

save_images() {
  prepare

  if [ -z "${IMAGE_TAG}" ]
  then
    echo "Missing IMAGE_TAG."
    exit 1
  fi

  export GORELEASER_CURRENT_TAG=${IMAGE_TAG}
  goreleaser release --rm-dist --snapshot --skip-publish

  mkdir -p "${IMAGE_SAVE_LOAD_DIR}"

  # Save images
  IMAGE_FILE_NAME_PREFIX=$(echo "${IMAGE_REPOSITORY}" | tr "/" "-")
  docker save ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-amd64 > ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-amd64.tar
  docker save ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-arm64 > ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-arm64.tar
  docker save ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-armv7 > ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-armv7.tar

  TEST_FILE_NAME=$(echo "${TEST_IMAGE_REPOSITORY}" | tr "/" "-")
  docker save ${IMAGE_REGISTRY}/${TEST_IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG} > ${IMAGE_SAVE_LOAD_DIR}/${TEST_FILE_NAME}.tar
}

load_and_push_images() {
  prepare
  if [ -z "${IMAGE_TAG}" ]
  then
    echo "Missing IMAGE_TAG."
    exit 1
  fi

  export GORELEASER_CURRENT_TAG=${IMAGE_TAG}

  # Load images
  IMAGE_FILE_NAME_PREFIX=$(echo "${IMAGE_REPOSITORY}" | tr "/" "-")
  docker load --input ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-amd64.tar
  docker load --input ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-arm64.tar
  docker load --input ${IMAGE_SAVE_LOAD_DIR}/${IMAGE_FILE_NAME_PREFIX}-armv7.tar

  TEST_FILE_NAME=$(echo "${TEST_IMAGE_REPOSITORY}" | tr "/" "-")
  docker load --input ${IMAGE_SAVE_LOAD_DIR}/${TEST_FILE_NAME}.tar

	# Push images
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-amd64
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-arm64
  docker push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-armv7
  docker push ${IMAGE_REGISTRY}/${TEST_IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}

  # Create manifest
  docker manifest create ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG} \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-amd64 \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-arm64 \
    --amend ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}-armv7
  docker manifest push ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${GORELEASER_CURRENT_TAG}
}

build() {
  prepare
  docker run --rm --privileged \
    -v $PWD:/go/src/github.com/kubeshop/botkube \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -w /go/src/github.com/kubeshop/botkube \
    -e GORELEASER_CURRENT_TAG=v9.99.9-dev \
    -e ANALYTICS_API_KEY="${ANALYTICS_API_KEY}" \
    goreleaser/goreleaser release --rm-dist --snapshot --skip-publish
}

release() {
  prepare
  if [ -z ${GITHUB_TOKEN} ]
  then
    echo "Missing GITHUB_TOKEN."
    exit 1
  fi
  goreleaser release --parallelism=1 --rm-dist
}


usage() {
    cat <<EOM
Usage: ${0} [build|release|release_snapshot]
Where,
  build: Builds project with goreleaser without pushing images.
  release_snapshot: Builds project without publishing release. It builds and pushes BotKube image with v9.99.9-dev image tag.
  release: Makes and published release to GitHub
EOM
    exit 1
}

[ ${#@} -gt 0 ] || usage
case "${1}" in
  build)
    build
    ;;
  release)
    release
    ;;
  release_snapshot)
    release_snapshot
    ;;
  save_images)
    save_images
    ;;
  save_pr_image)
    save_pr_image
    ;;
  load_and_push_images)
    load_and_push_images
    ;;
esac
