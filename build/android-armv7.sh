#!/usr/bin/env bash
# Cross-build for android/arm (armeabi-v7a) via the Android NDK (r26+; tested r29).
#
# Prereqs: an unpacked NDK and ANDROID_NDK_HOME pointing at it.
#   export ANDROID_NDK_HOME=/path/to/android-ndk-r29
#
# minSdk = 24 (Android 7.0), providing native getifaddrs() without restricted NETLINK_ROUTE sockets on Android 11+.

: "${ANDROID_NDK_HOME:?set ANDROID_NDK_HOME to an unpacked Android NDK}"
API=24
TC="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"

TARGET=android-armv7
GOOS=android GOARCH=arm GOARM=7
CC="$TC/bin/armv7a-linux-androideabi${API}-clang"
CXX="$TC/bin/armv7a-linux-androideabi${API}-clang++"
B2_COMPILER=clang
B2_VARIANT=android32
B2_TOOLSET_CXX="$CXX"
# On Android 11+, NETLINK_ROUTE sockets are forbidden by SELinux, throwing
# "session error: (13 Permission denied) bind: Permission denied".
# Disable Netlink and use getifaddrs (available since API 24).
B2_FLAGS="target-os=android address-model=32 architecture=arm define=TORRENT_USE_NETLINK=0 define=TORRENT_USE_IFADDRS=1"
# See android-arm64.sh: anet's //go:linkname needs Go's checklinkname off.
EXTRA_GO_LDFLAGS="-checklinkname=0"
# See android-arm64.sh: link libc++ statically, the app ships no libc++_shared.
EXTRA_CGO_LDFLAGS="-static-libstdc++"
EXTRA_CGO_CXXFLAGS="-DTORRENT_USE_NETLINK=0 -DTORRENT_USE_IFADDRS=1"
# See android-arm64.sh: OpenSSL android Configure env + NDK cmake toolchain.
ANDROID_NDK_ROOT="$ANDROID_NDK_HOME"
OPENSSL_PATH="$TC/bin"
OPENSSL_EXTRA_ARGS="-D__ANDROID_API__=${API}"
CMAKE_CROSS_ARGS="-DCMAKE_TOOLCHAIN_FILE=$ANDROID_NDK_HOME/build/cmake/android.toolchain.cmake -DANDROID_ABI=armeabi-v7a -DANDROID_PLATFORM=android-${API}"

export TARGET GOOS GOARCH GOARM CC CXX B2_COMPILER B2_VARIANT B2_TOOLSET_CXX \
       B2_FLAGS EXTRA_GO_LDFLAGS EXTRA_CGO_LDFLAGS EXTRA_CGO_CXXFLAGS \
       ANDROID_NDK_ROOT OPENSSL_PATH OPENSSL_EXTRA_ARGS CMAKE_CROSS_ARGS

# shellcheck source=_deps.sh
. "$(dirname "$0")/_deps.sh"
cross_build
