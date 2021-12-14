.PHONY: dockerauth
dockerauth:
ifdef DOCKER_USERNAME
ifdef DOCKER_PASSWORD
	echo -n $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USERNAME) --password-stdin
endif
endif
