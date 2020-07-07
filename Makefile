TESTER_NAME=service-catalog-tester
HEALTHCHECK_NAME=health-proxy
TESTER_IMG=$(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)/$(TESTER_NAME)
HEALTHCHECK_IMG=$(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)/$(HEALTHCHECK_NAME)
TAG=$(DOCKER_TAG)
BINARY=$(TESTER_NAME)

.PHONY: build
build:
	./before-commit.sh ci

.PHONY: build-image
build-image:
	docker build -t $(TESTER_NAME):latest .
	docker build -t $(HEALTHCHECK_NAME):latest cmd/healthcheck

.PHONY: push-image
push-image:
	docker tag $(TESTER_NAME) $(TESTER_IMG):$(TAG)
	docker tag $(HEALTHCHECK_NAME) $(HEALTHCHECK_IMG):$(TAG)
	docker push $(TESTER_IMG):$(TAG)
	docker push $(HEALTHCHECK_IMG):$(TAG)

.PHONY: ci-pr
ci-pr: build build-image push-image

.PHONY: ci-master
ci-master: build build-image push-image

.PHONY: ci-release
ci-release: build build-image push-image

.PHONY: clean
clean:
	rm -f $(BINARY)
