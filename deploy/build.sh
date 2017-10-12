#!/bin/sh

go get github.com/Masterminds/glide
cd $GOPATH/src/sphere.software/objstore
$GOPATH/bin/glide install
go build -o /out/objstore sphere.software/objstore/cmd/objstore
