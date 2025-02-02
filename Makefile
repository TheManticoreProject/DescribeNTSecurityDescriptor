.PHONY: all build_docker compile

IMGNAME := build_golang_project

all: build_docker compile

build_docker:
	docker build -t $(IMGNAME):latest -f Dockerfile .

compile: build_docker
	docker run --rm -v "$(shell pwd):/workspace" -it $(IMGNAME) $(shell uname -s | tr "[:upper:]" "[:lower:]") $(shell uname -m)
