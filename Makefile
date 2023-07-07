 # Go parameters
GOCMD=go
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=kaleido-go
BINARY_UNIX=$(BINARY_NAME)-tux
BINARY_MAC=$(BINARY_NAME)-mac
BINARY_WIN=$(BINARY_NAME)-win

all: deps govulncheck build
# govulncheck
GOVULNCHECK := $(GOBIN)/govulncheck
.PHONY: govulncheck
govulncheck: ${GOVULNCHECK}
	./govulnchecktool.sh
${GOVULNCHECK}:
	${GOCMD} install golang.org/x/vuln/cmd/govulncheck@latest
build: 
		$(GOBUILD) -o $(BINARY_NAME) -v
clean: 
		$(GOCLEAN)
		rm -f $(BINARY_NAME)
		rm -f $(BINARY_UNIX)
run:
		$(GOBUILD) -o $(BINARY_NAME) -v ./...
		./$(BINARY_NAME)
deps:
		$(GOGET) github.com/ethereum/go-ethereum
		$(GOGET) github.com/sirupsen/logrus
		$(GOGET) github.com/spf13/cobra
		$(GOGET) github.com/alexcesaro/statsd

build-linux:
		GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v
build-mac:
		GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_MAC) -v
build-win:
		GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_WIN) -v
