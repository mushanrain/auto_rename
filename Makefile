.PHONY: all clean build-all build-cli build-gui local-gui

BINARY_NAME=auto_rename

all: build-all

build-all: build-cli local-gui

build-cli: \
	build-linux-amd64-cli \
	build-darwin-arm64-cli \
	build-windows-x64-cli

local-gui: \
	build-darwin-arm64-gui

build-linux-amd64-cli:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 .

build-darwin-arm64-cli:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 .

build-windows-x64-cli:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-x64.exe .

build-darwin-arm64-gui:
	GOOS=darwin GOARCH=arm64 go build -tags gui -o $(BINARY_NAME)-darwin-arm64-gui .

build-windows-x64-gui:
	GOOS=windows GOARCH=amd64 go build -tags gui -o $(BINARY_NAME)-windows-x64-gui.exe .

build-linux-amd64-gui:
	GOOS=linux GOARCH=amd64 go build -tags gui -o $(BINARY_NAME)-linux-amd64-gui .

clean:
	rm -f $(BINARY_NAME)-*
