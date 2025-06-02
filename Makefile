SERVICE=icc

build-dev:
	bash ../dev/scripts/makefile/build-service.sh $(SERVICE) dev

build-prod:
	bash ../dev/scripts/makefile/build-service.sh $(SERVICE) prod

build-test:
	bash ../dev/scripts/makefile/build-service.sh $(SERVICE) tests

run-tests:
	echo "ICC has no tests"