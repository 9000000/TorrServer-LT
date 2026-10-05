#!/usr/bin/env bash
# Cross-build for linux/arm (armv7, hard-float).
# Toolchain: gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf
#   sudo apt install gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf

TARGET=linux-armv7
GOOS=linux GOARCH=arm GOARM=7
CC=arm-linux-gnueabihf-gcc
CXX=arm-linux-gnueabihf-g++
B2_VARIANT=armv7
B2_TOOLSET_CXX=arm-linux-gnueabihf-g++
B2_FLAGS="architecture=arm address-model=32"
# Statically link libstdc++ and libgcc so the binary runs reliably across
# various Linux distributions without GLIBCXX version mismatch errors.
EXTRA_CGO_LDFLAGS="-static-libstdc++ -static-libgcc"

export TARGET GOOS GOARCH GOARM CC CXX B2_VARIANT B2_TOOLSET_CXX B2_FLAGS EXTRA_CGO_LDFLAGS

# shellcheck source=_deps.sh
. "$(dirname "$0")/_deps.sh"
cross_build
