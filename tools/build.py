#!/usr/bin/env python3
"""Build the ARMv7 runtime, vendor adapters and release ZIP from public sources."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def run(arguments, **kwargs):
    subprocess.run([str(arg) for arg in arguments], check=True, **kwargs)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def copy(source, target):
    if source.is_dir():
        shutil.copytree(source, target, dirs_exist_ok=True, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    else:
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", default="go", help="Go 1.26.4+ for the WFC runtime")
    p.add_argument("--hysteria-go", default="go", help="Go 1.25.6 recommended for the pinned Hysteria source")
    p.add_argument("--ndk", type=Path, default=Path(os.environ.get("ANDROID_NDK_HOME", "")))
    p.add_argument("--version", default="v0.3.0")
    p.add_argument("--android-sdk", type=Path, default=Path(os.environ.get("ANDROID_HOME", os.environ.get("ANDROID_SDK_ROOT", str(Path.home()/"Library/Android/sdk")))))
    p.add_argument("--test", action="store_true")
    args = p.parse_args()
    host = "darwin-x86_64" if platform.system() == "Darwin" else "linux-x86_64"
    compiler = args.ndk / f"toolchains/llvm/prebuilt/{host}/bin/armv7a-linux-androideabi21-clang"
    if not compiler.is_file():
        p.error("请用 --ndk 指定 Android NDK r27c (27.2.12479018) 根目录")
    payload = ROOT / "dist/payload"
    if payload.exists():
        shutil.rmtree(payload)
    (payload / "bin").mkdir(parents=True)
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", GOWORK="off")
    if args.test:
        run([args.go, "test", "-race", "-count=1", "./cmd/sp970-wfc"], cwd=ROOT, env=env)
        run(["python3", "-m", "unittest", "discover", "-s", "tests", "-v"], cwd=ROOT)
    target = env | {"GOOS": "linux", "GOARCH": "arm", "GOARM": "7", "CGO_ENABLED": "0"}
    run([args.go, "build", "-trimpath", "-buildvcs=false", "-ldflags", f"-s -w -X main.buildVersion={args.version}",
         "-o", payload / "bin/sp970-wfc", "./cmd/sp970-wfc"], cwd=ROOT, env=target)
    for name in ("sp970_uim_bridge", "sp970_ims_xfrm", "sp970_euicc_bridge"):
        run([compiler, "-std=c11", "-O2", "-Wall", "-Wextra", "-Werror", "-fPIE", "-pie",
             "-Wl,--hash-style=sysv", "-Wl,--no-rosegment", "-Wl,--build-id=none",
             ROOT / f"native/{name}.c", "-ldl", "-o", payload / f"bin/{name}"])
    sdk = args.android_sdk
    android_jar = sdk / "platforms/android-34/android.jar"
    d8 = sdk / "build-tools/34.0.0/d8"
    if not android_jar.is_file() or not d8.is_file():
        p.error("请安装 Android SDK platforms;android-34 与 build-tools;34.0.0，并传入 --android-sdk")
    java_home = Path(os.environ.get("JAVA_HOME", "/usr"))
    java_classes = ROOT / "dist/android-classes"
    java_classes.mkdir(parents=True, exist_ok=True)
    run([java_home/"bin/javac", "-source", "8", "-target", "8", "-Xlint:-options", "-cp", android_jar,
         "-d", java_classes, ROOT/"native/Sp970Android.java"])
    java_dex = ROOT / "dist/android-dex"
    java_dex.mkdir(parents=True, exist_ok=True)
    run([d8, "--min-api", "19", "--lib", android_jar, "--output", java_dex, java_classes/"Sp970Android.class"])
    copy(java_dex/"classes.dex", payload/"bin/sp970_android.dex")
    hy = ROOT / "third_party/hysteria/app"
    if "TableIndex:               20220" not in (hy / "internal/tun/server.go").read_text():
        raise SystemExit("Missing documented ARM32 TUN compatibility patch")
    hy_target = target.copy()
    run([args.hysteria_go, "build", "-trimpath", "-buildvcs=false", "-ldflags", "-s -w -X github.com/apernet/hysteria/app/v2/cmd.appVersion=v2.6.5-sp970tun1",
         "-o", payload / "bin/hysteria", "."], cwd=hy, env=hy_target)
    for relative in ("setup.py", "tools", "web", "service", "examples", "data", "LICENSE", "NOTICE", "README.md", "docs"):
        copy(ROOT / relative, payload / relative)
    # Include the exact corresponding source of both binaries in each release,
    # including pinned modules and their original license notices.
    source = payload / "source"
    for relative in ("cmd", "native", "third_party", "patches", "go.mod", "go.sum", "LICENSE", "NOTICE", "tools", "tests", "setup.py", "service", "web", "examples", "extras", "data", "README.md", "SECURITY.md", "docs"):
        copy(ROOT / relative, source / relative)
    licenses = payload / "THIRD_PARTY_LICENSES"
    licenses.mkdir(exist_ok=True)
    modules = "\n".join(subprocess.check_output([toolchain, "list", "-m", "-json", "all"], cwd=directory, env=env, text=True)
                         for toolchain, directory in ((args.go, ROOT), (args.hysteria_go, hy)))
    decoder = json.JSONDecoder(); offset = 0
    while offset < len(modules):
        while offset < len(modules) and modules[offset].isspace(): offset += 1
        if offset >= len(modules): break
        module, length = decoder.raw_decode(modules, offset); offset = length
        directory = module.get("Replace", module).get("Dir") or module.get("Dir")
        if not directory: continue
        path = Path(directory)
        for file in path.iterdir():
            if file.is_file() and file.name.upper().startswith(("LICENSE", "COPYING", "NOTICE")):
                name = module["Path"].replace("/", "_") + "_" + file.name
                copy(file, licenses / name)
    manifest = {"version": args.version, "platform": "android-armv7", "sdk_tested": 19,
                "go": subprocess.check_output([args.go, "version"], text=True).strip(),
                "hysteria_go": subprocess.check_output([args.hysteria_go, "version"], text=True).strip(),
                "binaries": {file.name: digest(file) for file in (payload / "bin").iterdir()}}
    (payload / "build-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    entries = sorted(file for file in payload.rglob("*") if file.is_file())
    (payload / "SHA256SUMS").write_text("".join(f"{digest(file)}  {file.relative_to(payload).as_posix()}\n" for file in entries))
    archive = ROOT / f"dist/sp970-wfc-{args.version}-android-armv7.zip"
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        for file in sorted(payload.rglob("*")):
            if file.is_file():
                info = zipfile.ZipInfo(file.relative_to(payload).as_posix(), date_time=(2026, 1, 1, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                info.external_attr = (0o100755 if file.parent.name == "bin" or file.suffix == ".sh" else 0o100644) << 16
                z.writestr(info, file.read_bytes())
    (archive.parent / "SHA256SUMS").write_text(f"{digest(archive)}  {archive.name}\n")
    print(json.dumps({"archive": str(archive.relative_to(ROOT)), "sha256": digest(archive)}))


if __name__ == "__main__":
    main()
