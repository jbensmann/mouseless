#!/bin/sh
targets="linux,amd64"

rm -rf dist/

for i in $targets; do
    IFS=","
    set -- $i
    os=$1
    arch=$2
    echo "building $os:$arch"
    GOOS=$os GOARCH=$arch go build -ldflags="-extldflags=-static" -o dist/mouseless .
    if [ $? != 0 ]; then
        exit 1
    fi
    tar -czvf dist/mouseless-${os}-${arch}.tar.gz dist/mouseless
done


