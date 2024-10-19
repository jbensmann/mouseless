#!/bin/sh
rm -rf dist/

# read the version from the git tag and remove the 'v' prefix
# if there is not tag, use the short commit hash
VERSION=$(git describe --tags --abbrev=0 | sed 's/^v//')
if [ -z "$VERSION" ]; then
    VERSION=$(git rev-parse --short HEAD)
fi
echo "VERSION=${VERSION}"

build() {
    os=$1
    arch=$2
    echo "building $os:$arch"
    GOOS=$os GOARCH=$arch go build -ldflags "-s -w -X main.version=$VERSION" -o dist/mouseless .
    if [ $? != 0 ]; then
        exit 1
    fi
    tar -czvf "dist/mouseless-${os}-${arch}.tar.gz" dist/mouseless
}

build linux amd64
