# Build parameters
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GO_BUILD_FLAGS = -v

# Project information
PROJECT_NAME := tiup-checker
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT_HASH ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Directory structure
ROOT_DIR := $(shell pwd)
BUILD_DIR := $(ROOT_DIR)/build
DIST_DIR := $(ROOT_DIR)/dist
LOG_DIR := $(ROOT_DIR)/logs

# Frontend configuration
FRONTEND_DIR := web

# Binary files
SERVER_BINARY := $(BUILD_DIR)/server
CHECKER_BINARY := $(BUILD_DIR)/checker


# Go build flags
LD_FLAGS := -X main.Version=$(VERSION) \
            -X main.CommitHash=$(COMMIT_HASH) \
            -X main.BuildTime=$(BUILD_TIME)

# Color output
BLUE := \033[34m
GREEN := \033[32m
RED := \033[31m
YELLOW := \033[33m
NC := \033[0m # No Color

.PHONY: all build clean test lint docker help frontend-* dev

## Default target
all: build frontend-build ## Build backend and frontend

update-image: server-docker server-image-push frontend-docker frontend-image-push ## Update server and frontend docker images

## Build-related targets
build: prepare $(SERVER_BINARY) $(CHECKER_BINARY) ## Build backend binaries

$(SERVER_BINARY): ## Build server
	@printf "$(BLUE)Building server binary...$(NC)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) \
		-ldflags "$(LD_FLAGS)" \
		-o $@ \
		./cmd/server
	@printf "$(GREEN)Server binary built successfully$(NC)\n"

$(CHECKER_BINARY): ## Build checker
	@printf "$(BLUE)Building checker binary...$(NC)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		$(GO_BUILD_FLAGS) \
		-ldflags "$(LD_FLAGS)" \
		-o $@ \
		./cmd/checker
	@printf "$(GREEN)Checker binary built successfully$(NC)\n"

prepare: ## Prepare build environment
	@mkdir -p $(BUILD_DIR) $(DIST_DIR) $(LOG_DIR)
	@printf "$(GREEN)Build directories created$(NC)\n"

server-docker: ## Build server for docker
	@printf "$(BLUE)Building server for docker...$(NC)\n"
	docker build -t hub.pingcap.net/jenkins/check-tiup-nightly:server .
	@printf "$(GREEN)Server docker image built successfully$(NC)\n"

server-image-push: ## Push server docker image
	@printf "$(BLUE)Pushing server docker image...$(NC)\n"
	docker push hub.pingcap.net/jenkins/check-tiup-nightly:server
	@printf "$(GREEN)Server docker image pushed successfully$(NC)\n"

## Frontend-related targets
frontend-install: ## Install frontend dependencies
	@printf "$(BLUE)Installing frontend dependencies...$(NC)\n"
	cd $(FRONTEND_DIR) && npm install
	@printf "$(GREEN)Frontend dependencies installed$(NC)\n"

frontend-dev: ## Run frontend development server
	@printf "$(BLUE)Starting frontend development server...$(NC)\n"
	cd $(FRONTEND_DIR) && \
	API_BASE_URL=$(API_BASE_URL) \
	NODE_ENV=development \
	npm run dev

frontend-build: ## Build frontend for production
	@printf "$(BLUE)Building frontend for production...$(NC)\n"
	cd $(FRONTEND_DIR) && \
	API_BASE_URL=$(API_BASE_URL) \
	NODE_ENV=production \
	npm run build
	@printf "$(GREEN)Frontend built successfully$(NC)\n"

frontend-docker: ## Build frontend for docker
	@printf "$(BLUE)Building frontend for docker...$(NC)\n"
	cd $(FRONTEND_DIR) && \
	docker build -t hub.pingcap.net/jenkins/check-tiup-nightly:web .
	@printf "$(GREEN)Frontend docker image built successfully$(NC)\n"

frontend-image-push: ## Push frontend docker image
	@printf "$(BLUE)Pushing frontend docker image...$(NC)\n"
	docker push hub.pingcap.net/jenkins/check-tiup-nightly:web
	@printf "$(GREEN)Frontend docker image pushed successfully$(NC)\n"

frontend-clean: ## Clean frontend build files
	@printf "$(BLUE)Cleaning frontend build files...$(NC)\n"
	rm -rf $(FRONTEND_DIR)/.next
	rm -rf $(FRONTEND_DIR)/node_modules
	rm -rf $(FRONTEND_DIR)/.cache
	@printf "$(GREEN)Frontend cleaned$(NC)\n"

dev-server: build ## Start backend development server
	@printf "$(BLUE)Starting backend server...$(NC)\n"
	$(SERVER_BINARY)


## Test-related targets
test: test-backend

test-backend: ## Run backend tests
	@printf "$(BLUE)Running backend tests...$(NC)\n"
	go test -v -race -cover ./...
	@printf "$(GREEN)Backend tests completed$(NC)\n"

## Cleanup-related targets
clean: clean-backend clean-frontend ## Clean all build files

clean-backend: ## Clean backend build files
	@printf "$(BLUE)Cleaning backend build files...$(NC)\n"
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	@printf "$(GREEN)Backend cleaned$(NC)\n"

clean-frontend: ## Clean frontend build files
	@printf "$(BLUE)Cleaning frontend build files...$(NC)\n"
	make frontend-clean
	@printf "$(GREEN)Frontend cleaned$(NC)\n"

## Run-related targets
run-server: $(SERVER_BINARY) ## Run server
	@printf "$(BLUE)Starting server...$(NC)\n"
	$(SERVER_BINARY)

run-checker: $(CHECKER_BINARY) ## Run checker
	@printf "$(BLUE)Starting checker...$(NC)\n"
	$(CHECKER_BINARY)

## Help
help: ## Show help information
	@printf "$(BLUE)Available targets:$(NC)\n"
	@printf "\n$(YELLOW)Usage:$(NC)\n"
	@printf "  make $(GREEN)<target>$(NC)\n\n"
	@printf "$(YELLOW)Targets:$(NC)\n"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf "\n$(YELLOW)Frontend-specific targets:$(NC)\n"
	@printf "  $(GREEN)frontend-install$(NC)    Install frontend dependencies\n"
	@printf "  $(GREEN)frontend-dev$(NC)        Run frontend development server\n"
	@printf "  $(GREEN)frontend-build$(NC)      Build frontend for production\n"
	@printf "  $(GREEN)frontend-lint$(NC)       Run frontend code linting\n"
	@printf "  $(GREEN)frontend-test$(NC)       Run frontend tests\n"
	@printf "  $(GREEN)frontend-clean$(NC)      Clean frontend build files\n"

# Set default target
.DEFAULT_GOAL := help
