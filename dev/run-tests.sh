#!/bin/bash

# Executes all tests. Should errors occur, CATCH will be set to 1, causing an erroneous exit code.

echo "########################################################################"
echo "###################### Run Tests and Linters ###########################"
echo "########################################################################"

# Setup
LOCAL_PWD=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
IMAGE_TAG=openslides-icc-tests

# Safe Exit
trap 'docker stop manage-test' EXIT

# Execution
make build-test
docker run --privileged -t ${IMAGE_TAG} --name manage-test ./dev/container-tests.sh

# Linters
bash "$LOCAL_PWD"/run-lint.sh -s -c