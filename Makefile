 # Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=kaleido-go
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_MAC=$(BINARY_NAME)_mac

all: deps build
build: 
		$(GOBUILD) -o $(BINARY_NAME) -v cmd/kaleido-go.go
clean: 
		$(GOCLEAN)
		rm -f $(BINARY_NAME)
		rm -f $(BINARY_UNIX)
run:
		$(GOBUILD) -o $(BINARY_NAME) -v ./...
		./$(BINARY_NAME)
deps:
		$(GOGET) github.com/ethereum/go-ethereum/accounts/abi
		$(GOGET) github.com/ethereum/go-ethereum/common
		$(GOGET) github.com/ethereum/go-ethereum/core/types
		$(GOGET) github.com/ethereum/go-ethereum/crypto
		$(GOGET) github.com/sirupsen/logrus
		$(GOGET) github.com/spf13/cobra
		$(GOGET) github.com/spf13/viper
		$(GOGET) github.com/mitchellh/go-homedir

build-linux:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v
build-mac:
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_MAC) -v
