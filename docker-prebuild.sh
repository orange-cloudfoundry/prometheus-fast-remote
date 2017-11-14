#!/bin/bash
# DEBUG
#set -xv

CURDIR=$(pwd)
BUILDDIR=$CURDIR/build
IMGNAME=build-adapter

# Clean
rm $BUILDDIR/bin/* 2>/dev/null >/dev/null

docker build --no-cache -t noh/${IMGNAME} -f build/Dockerfile .
docker run --rm --name $IMGNAME -v $BUILDDIR/bin:/go/bin noh/${IMGNAME}
