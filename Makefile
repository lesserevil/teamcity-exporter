.PHONY: info build
GO = go
PROMU = promu

info:
	@echo "build: Go build"
	@echo "promu: Promu download"

build:
	@$(PROMU) build

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		$(GO) get -u github.com/prometheus/promu
