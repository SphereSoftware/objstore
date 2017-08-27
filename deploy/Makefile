all:

build:
	go get github.com/Masterminds/glide
	cd $GOPATH/sphere.software/objstore
	glide install
	go build
	docker build -t SphereSoftware/objstore .

OUT ?= $(shell pwd)/out
PROJECT ?= $(GOPATH)/src/sphere.software/objstore

local:
	mkdir -p $(OUT)
	docker run -a stdout -a stderr --rm \
		-v $(OUT):/out -v $(PROJECT):/go/src/sphere.software/objstore -v $(GOPATH):/go/local \
		-e GOPATH=/go:/go/local \
		golang:alpine go build -o /out/objstore sphere.software/objstore/cmd/objstore
	docker build -t SphereSoftware/objstore .
	rm $(OUT)/objstore
	rmdir $(OUT)
