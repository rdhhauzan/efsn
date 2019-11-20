# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: all test lint clean devtools
.PHONY: efsn debug bootnode

GOBIN = $(shell pwd)/build/bin
GO ?= latest

# to prevent mistakely run 'bash Makefile',
# which will execute dangerous command 'rm /*'
ifneq ($(GOBIN),)
endif

efsn:
	build/env.sh go run build/ci.go install ./cmd/efsn
	@echo "Done building."
	@echo "Run \"$(GOBIN)/efsn\" to launch efsn."

debug:
	# https://ethereum.stackexchange.com/questions/41489/how-to-debug-geth-with-delve?rq=1
	@echo building debug version
	build/env.sh go build -o ./build/bin/efsn   -gcflags=all='-N -l' -v ./cmd/efsn
	@echo end building debug version
	@echo "Run \"$(GOBIN)/efsn\" to launch efsn."

bootnode:
	build/env.sh go run build/ci.go install ./cmd/bootnode

rlpdump:
	build/env.sh go run build/ci.go install ./cmd/rlpdump

ethkey:
	build/env.sh go run build/ci.go install ./cmd/ethkey

all:
	build/env.sh go run build/ci.go install

test: all
	build/env.sh go run build/ci.go test

lint: ## Run linters.
	build/env.sh go run build/ci.go lint

clean:
	./build/clean_go_build_cache.sh
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'
