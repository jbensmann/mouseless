#!/bin/sh
targets="linux,amd64"
for i in $targets; do
    IFS=","
    set -- $i
    os=$1
    arch=$2
    echo "building $os:$arch"
    GOOS=$os GOARCH=$arch go build -ldflags="-extldflags=-static" -o dist/${os}_${arch}/ .
    if [ $? != 0 ]; then
        exit 1
    fi
done

rm -f mouseless.zip
zip -r mouseless.zip dist

