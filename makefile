.PHONY: start build

NOW = $(shell date -u '+%Y%m%d%I%M%S')

APP = xrayw
SERVER_BIN = ./dist/$(APP)

dev: start

# 初始化mod
init:
	go mod init ${APP}

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags "-w -s" -o $(SERVER_BIN) ./
build-win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o $(SERVER_BIN).exe ./

# 运行 -gcflags=-G=3
start:
	go run main.go --port 8199

snone:
	go run main.go --port 8199 -c none -print

