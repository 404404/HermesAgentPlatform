PROJECT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
COMPOSE := docker compose -f $(PROJECT_DIR)deploy/docker-compose.yml --project-directory $(PROJECT_DIR)

.PHONY: up down logs build test status
up:
	DOCKER_BUILDKIT=0 $(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=200

build:
	$(COMPOSE) build

test:
	cd $(PROJECT_DIR)backend && GOPROXY=$${GOPROXY:-https://mirrors.aliyun.com/goproxy/} go test ./...

status:
	$(COMPOSE) ps
