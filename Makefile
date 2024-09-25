build-dev:
	rm -fr openslides-autoupdate-service
	cp -r ../openslides-autoupdate-service .
	docker build . --target development --tag openslides-icc-dev
	rm -fr openslides-autoupdate-service
