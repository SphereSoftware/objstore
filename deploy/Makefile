all:

build:
	go get github.com/Masterminds/glide
	cd $(GOPATH)/src/sphere.software/objstore
	glide install
	go build
	docker build -t spheresoftware/objstore .

OUT ?= $(shell pwd)/out
PROJECT ?= $(GOPATH)/src/sphere.software/objstore

local:
	mkdir -p $(OUT)
	docker run -a stdout -a stderr --rm \
		-v $(OUT):/out -v $(PROJECT):/go/src/sphere.software/objstore \
		-e GOPATH=/go \
		golang:alpine /go/src/sphere.software/objstore/deploy/build.sh
	docker build -t spheresoftware/objstore .
	rm $(OUT)/objstore
	rmdir $(OUT)

docker:
	make local
	docker save -o objstore.tar spheresoftware/objstore
	zip objstore.tar.zip objstore.tar
	