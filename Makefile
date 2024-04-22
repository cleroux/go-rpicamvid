NAME := rpicamvid-server

.PHONY: build
build:
	go build -o $(NAME) cmd/server/main.go

