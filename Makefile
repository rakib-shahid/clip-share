# Docker Hub image settings. Override these on the command line or in an
# uncommitted .env.make file; never commit credentials here.
DOCKERHUB_USERNAME ?= your-dockerhub-username
IMAGE_NAME ?= clip-share
VERSION ?= 0.0.1
IMAGE ?= $(DOCKERHUB_USERNAME)/$(IMAGE_NAME)
PLATFORMS ?= linux/amd64

.PHONY: image publish login release check test compose-pull

define require-dockerhub-user
$(if $(filter your-dockerhub-username,$(DOCKERHUB_USERNAME)),$(error Set DOCKERHUB_USERNAME, for example: make release DOCKERHUB_USERNAME=rakibshahid VERSION=2026.09.04))
endef

check:
	docker buildx version
	docker version

test:
	go vet ./...
	go test ./...
	npm.cmd --prefix web run lint
	npm.cmd --prefix web run test
	npm.cmd --prefix web run build

image:
	$(call require-dockerhub-user)
	docker buildx build --platform $(PLATFORMS) --target production -t $(IMAGE):$(VERSION) . --load

login:
	docker login

publish: login
	$(call require-dockerhub-user)
	docker buildx build --platform $(PLATFORMS) --target production -t $(IMAGE):$(VERSION) -t $(IMAGE):latest . --push

release: publish
	@echo Published $(IMAGE):$(VERSION) and $(IMAGE):latest

compose-pull:
	docker compose --env-file .env -f compose.komodo.yaml pull
