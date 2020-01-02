SHELL := /bin/sh

BINARY_NAME ?= gcp-quota-exporter
DOCKER_REGISTRY ?= mintel
DOCKER_IMAGE = ${DOCKER_REGISTRY}/${BINARY_NAME}

VERSION ?= $(shell echo `git symbolic-ref -q --short HEAD || git describe --tags --exact-match` | tr '[/]' '-')
DOCKER_TAG ?= ${VERSION}

ARTIFACTS = /tmp/artifacts

build : gcp-quota-exporter
.PHONY : build

gcp-quota-exporter : main.go
	@echo "building go binary"
	@CGO_ENABLED=0 GOOS=linux go build .

test : check-env
	@if [[ ! -d ${ARTIFACTS} ]]; then \
		mkdir ${ARTIFACTS}; \
	fi
	go test -v -coverprofile=c.out
	go tool cover -html=c.out -o coverage.html
	mv coverage.html /tmp/artifacts
.PHONY : test

check-env :
ifndef GCP_PROJECT
	$(error GCP_PROJECT is undefined)
endif
.PHONY : check-env