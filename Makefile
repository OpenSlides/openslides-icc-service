build-dev:
	docker build . --target development --tag openslides-icc-dev

build-dev-fullstack:
	DOCKER_BUILDKIT=1 docker build . --target development-fullstack --build-context autoupdate=../openslides-autoupdate-service --tag openslides-icc-dev-fullstack
