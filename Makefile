override SERVICE=icc
override MAKEFILE_PATH=../dev/scripts/makefile
override DOCKER_COMPOSE_FILE=

# Build images for different contexts

build build-prod build-dev build-tests:
	bash $(MAKEFILE_PATH)/make-build-service.sh $@ $(SERVICE)

# Development

.PHONY: run-dev%

run-dev%:
	bash $(MAKEFILE_PATH)/make-run-dev.sh "$@" "$(SERVICE)" "$(DOCKER_COMPOSE_FILE)" "$(ARGS)" "$(USED_SHELL)"

# Tests

run-tests:
	bash dev/run-tests.sh

run-lint:
	bash dev/run-lint.sh -l