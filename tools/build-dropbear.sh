#!/bin/sh
# Build the public-key-only SSH component for SP970-B (Android 4.4, ARMv7).
# Usage: ANDROID_NDK_HOME=/path/to/ndk ./tools/build-dropbear.sh SOURCE_DIR OUTPUT
# SOURCE_DIR must contain the official Dropbear 2026.94 release tarball contents.
set -eu
PATCH_FILE=$(cd "$(dirname "$0")/../patches" && pwd)/dropbear-android.patch
SOURCE=$(cd "$1" && pwd)
OUTPUT=$2
case "$OUTPUT" in /*) ;; *) OUTPUT="$PWD/$OUTPUT";; esac
: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to Android NDK r27c}"
TOOLCHAIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-x86_64/bin"
export CC="$TOOLCHAIN/armv7a-linux-androideabi21-clang"
export AR="$TOOLCHAIN/llvm-ar"
export RANLIB="$TOOLCHAIN/llvm-ranlib"
export CFLAGS='-Os -fPIE'
cd "$SOURCE"
cat > localoptions.h <<'EOF'
/* Cross compilation cannot probe Android's Unix98 PTYs. */
#define USE_DEV_PTMX 1
#define DROPBEAR_SVR_PASSWORD_AUTH 0
#define DROPBEAR_SVR_PAM_AUTH 0
#define DROPBEAR_DSS 0
#define DROPBEAR_SFTPSERVER 0
#define DEFAULT_PATH "/system/bin:/system/xbin:/data/adb/sp970-wfc/bin"
#define DEFAULT_ROOT_PATH "/system/bin:/system/xbin:/data/adb/sp970-wfc/bin"
#define DROPBEAR_PIDFILE "/data/adb/sp970-wfc/run/ssh.pid"
#define DO_MOTD 0
EOF
if ! grep -q SP970_ANDROID_SHELL src/common-session.c; then
    patch -p1 < "$PATCH_FILE"
fi
# Static ARM Bionic needs native ELF TLS alignment of at least 32 bytes.
printf '%s\n' '__thread char sp970_tls_alignment __attribute__((aligned(32)));' > sp970-tls-align.c
"$CC" $CFLAGS -fno-emulated-tls -c sp970-tls-align.c -o sp970-tls-align.o
./configure --host=arm-linux-androideabi --enable-static --disable-zlib \
    --disable-syslog --disable-lastlog --disable-utmp --disable-utmpx \
    --disable-wtmp --disable-wtmpx --disable-loginfunc
make -j4 PROGRAMS='dropbear dropbearkey scp' MULTI=1 \
    LDFLAGS="$SOURCE/sp970-tls-align.o -Wl,-u,sp970_tls_alignment -static"
"$TOOLCHAIN/llvm-strip" dropbearmulti
cp dropbearmulti "$OUTPUT"
