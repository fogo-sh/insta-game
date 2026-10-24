"""Drive a real graphical game client on an isolated X display with synthetic input."""

import argparse
import hashlib
import json
import os
import pathlib
import shutil
import signal
import subprocess
import time

p = argparse.ArgumentParser()
p.add_argument("game")
p.add_argument("address")
p.add_argument("--out", type=pathlib.Path, required=True)
p.add_argument("--seconds", type=int, default=240)
p.add_argument("--robots", type=int, default=0)
a = p.parse_args()
a.out = a.out.resolve()
a.out.mkdir(parents=True, exist_ok=False)
fixtures = pathlib.Path(
    os.environ.get(
        "INSTA_GAME_FIXTURES", pathlib.Path.home() / ".local/share/insta-game/fixtures"
    )
)
b = fixtures / "insta-game-all-loading"
env = dict(
    os.environ,
    LIBGL_ALWAYS_SOFTWARE="1",
    LP_NUM_THREADS="1",
    SDL_AUDIODRIVER="dummy",
    XDG_CACHE_HOME=str(a.out / "cache"),
    XDG_CONFIG_HOME=str(a.out / "config"),
    XDG_DATA_HOME=str(a.out / "user"),
    TMPDIR=str(a.out),
    LD_LIBRARY_PATH=str(b / "client-runtime/usr/lib/x86_64-linux-gnu")
    + ":"
    + str(
        fixtures / "insta-game-validation/limbo-client/runtime/usr/lib/x86_64-linux-gnu"
    ),
)
client = x = None
private_prefix = None


def stop(*_):
    raise SystemExit(0)


def generator_sample():
    memory = {
        key: int(value.split()[0]) * 1024
        for key, value in (
            line.split(":", 1)
            for line in pathlib.Path("/proc/meminfo").read_text().splitlines()
        )
    }
    pressure = pathlib.Path("/proc/pressure/memory").read_text().splitlines()[1]
    memory_pressure = float(
        dict(part.split("=") for part in pressure.split()[1:])["avg10"]
    )
    result = {
        "memory_full_avg10": memory_pressure,
        "time": time.time(),
        "memory_available": memory["MemAvailable"],
        "load1": float(pathlib.Path("/proc/loadavg").read_text().split()[0]),
        "logical_cpus": os.cpu_count(),
        "cpu_ticks": list(
            map(int, pathlib.Path("/proc/stat").read_text().splitlines()[0].split()[1:])
        ),
    }
    if client and client.poll() is None:
        stat = pathlib.Path("/proc", str(client.pid), "stat").read_text().split()
        result.update(
            client_cpu_ticks=int(stat[13]) + int(stat[14]),
            client_rss_bytes=int(stat[23]) * os.sysconf("SC_PAGE_SIZE"),
        )
    return result


signal.signal(signal.SIGTERM, stop)
try:
    initial = generator_sample()
    if initial["memory_available"] < 2 * 1024**3 or initial["memory_full_avg10"] > 5:
        raise RuntimeError("load generator lacks 2 GiB headroom or has memory pressure")
    x = subprocess.Popen(
        ["Xvfb", "-displayfd", "1", "-screen", "0", "640x480x24", "-nolisten", "tcp"],
        stdout=subprocess.PIPE,
        stderr=open(a.out / "xvfb.log", "w"),
        start_new_session=True,
    )
    env["DISPLAY"] = ":" + x.stdout.readline().decode().strip()
    name = "Load" + a.out.name.replace("-", "")[-12:]
    if a.game == "fteqw":
        env.update(
            LD_LIBRARY_PATH=str(
                fixtures
                / "insta-game-validation/limbo-client/vkquake/squashfs-root/usr/lib"
            ),
            VK_ICD_FILENAMES="/usr/share/vulkan/icd.d/lvp_icd.json",
            VK_DRIVER_FILES="/usr/share/vulkan/icd.d/lvp_icd.json",
            SDL_VIDEODRIVER="x11",
        )
        cmd = [
            str(
                fixtures
                / "insta-game-validation/limbo-client/vkquake/squashfs-root/usr/bin/vkquake"
            ),
            "-basedir",
            str(fixtures / "insta-game-validation/shared-host/assets"),
            "-window",
            "-width",
            "320",
            "-height",
            "240",
            "-nosound",
            "+name",
            name,
            "+host_maxfps",
            "30",
            "+connect",
            a.address,
        ]
    elif a.game == "q2repro":
        private_prefix = a.out / "wine-prefix"
        subprocess.run(
            [
                "cp",
                "-a",
                "--reflink=auto",
                str(b / "q2-wine-prefix"),
                str(private_prefix),
            ],
            check=True,
        )
        env.update(
            WINEPREFIX=str(private_prefix),
            WINEDEBUG="-all",
            WINEDLLOVERRIDES="mscoree,mshtml=",
        )
        cmd = [
            str(
                pathlib.Path.home()
                / ".local/share/Steam/steamapps/common/Proton - Experimental/files/bin/wine"
            ),
            str(b / "q2repro-client/q2repro.exe"),
            "+set",
            "basedir",
            "Z:" + str(b / "q2-data"),
            "+set",
            "libdir",
            "Z:" + str(b / "q2-data"),
            "+set",
            "homedir",
            "Z:" + str(a.out / "q2-user"),
            "+set",
            "vid_fullscreen",
            "0",
            "+set",
            "vid_geometry",
            "320x240",
            "+set",
            "s_enable",
            "0",
            "+set",
            "cl_maxfps",
            "30",
            "+set",
            "name",
            name,
            "+set",
            "allow_download",
            "0",
            "+set",
            "qport",
            str(10001 + int(a.out.name.rsplit("-", 1)[-1])),
            "+connect",
            a.address,
        ]
    elif a.game in ["ioquake3", "openarena"]:
        base = (
            ["+set", "fs_basepath", str(b / "q3-data")]
            if a.game == "ioquake3"
            else [
                "+set",
                "com_basegame",
                "baseoa",
                "+set",
                "com_legacyprotocol",
                "71",
                "+set",
                "com_protocol",
                "71",
                "+set",
                "fs_basepath",
                str(
                    fixtures
                    / "insta-game-release-validation/runtime/usr/share/games/openarena"
                ),
            ]
        )
        binary = str(b / "client-runtime/usr/lib/ioquake3/ioquake3")
        cmd = [
            binary,
            *base,
            "+set",
            "fs_homepath",
            str(a.out / "home"),
            "+set",
            "r_fullscreen",
            "0",
            "+set",
            "r_mode",
            "-1",
            "+set",
            "r_customwidth",
            "320",
            "+set",
            "r_customheight",
            "240",
            "+set",
            "com_maxfps",
            "30",
            "+set",
            "s_initsound",
            "0",
            "+set",
            "vm_cgame",
            "0",
            "+set",
            "vm_ui",
            "0",
            "+set",
            "cl_allowDownload",
            "0",
            "+set",
            "name",
            name,
            "+connect",
            a.address,
        ]
    elif a.game == "xonotic":
        cmd = [
            str(b / "xonotic-client/Xonotic/xonotic-linux64-sdl"),
            "-basedir",
            str(fixtures / "insta-game-validation/Xonotic"),
            "-userdir",
            str(a.out / "home"),
            "-window",
            "-width",
            "320",
            "-height",
            "240",
            "-nosound",
            "+set",
            "cl_maxfps",
            "30",
            "+set",
            "_cl_name",
            name,
            "+set",
            "cl_allow_uid2name",
            "0",
            "+connect",
            a.address,
        ]
    elif a.game == "ut99":
        config = (
            (b / "ut99/System/Default.ini")
            .read_text()
            .replace(
                "ViewportManager=WinDrv.WindowsClient",
                "ViewportManager=SDLDrv.SDLClient",
            )
            .replace("WindowedViewportX=1024", "WindowedViewportX=320")
            .replace("WindowedViewportY=768", "WindowedViewportY=240")
            .replace("FirstRun=0", "FirstRun=469")
            .replace("FrameRateLimit=0", "FrameRateLimit=30")
            .replace("[SDLDrv.SDLClient]", "[SDLDrv.SDLClient]\nFrameRateLimit=30")
        )
        (a.out / "UnrealTournament.ini").write_text(config)
        env["LD_LIBRARY_PATH"] = str(b / "ut99/System") + ":" + env["LD_LIBRARY_PATH"]
        cmd = [
            str(b / "ut99/System/ut-bin-amd64"),
            a.address + "?Name=" + name,
            "-windowed",
            "-nosound",
            "-nohomedir",
            "ini=" + str(a.out / "UnrealTournament.ini"),
        ]
    elif a.game == "bzflag":
        cmd = [
            str(b / "client-runtime/usr/games/bzflag"),
            "-window",
            "320x240",
            "-configdir",
            str(a.out / "home"),
            "-directory",
            str(b / "client-runtime/usr/share/games/bzflag"),
            "-nolist",
            "-mute",
            name + "@" + a.address,
        ]
    else:
        raise ValueError(a.game)
    if a.game == "bzflag" and a.robots:
        cmd[1:1] = ["-solo", str(a.robots)]
    (a.out / "client-info.json").write_text(
        json.dumps(
            {
                "executable": cmd[1] if a.game == "q2repro" else cmd[0],
                "executable_sha256": hashlib.sha256(
                    pathlib.Path(cmd[1] if a.game == "q2repro" else cmd[0]).read_bytes()
                ).hexdigest(),
                "private_display": True,
                "isolated_wine_prefix": bool(private_prefix),
                "explicit_qport": (10001 + int(a.out.name.rsplit("-", 1)[-1]))
                if a.game == "q2repro"
                else None,
                "scripted_input": True,
                "ut_sdl_frame_cap": 30 if a.game == "ut99" else None,
                "configured_frame_cap": 30 if a.game != "bzflag" else None,
                "robots": a.robots,
            }
        )
    )
    (a.out / "launch.json").write_text(json.dumps(cmd))
    (a.out / "display").write_text(env["DISPLAY"])
    client = subprocess.Popen(
        cmd,
        cwd=(b / "ut99/System" if a.game == "ut99" else a.out),
        env=env,
        stdout=open(a.out / "client.log", "w"),
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    (a.out / "pids.json").write_text(
        json.dumps({"client": client.pid, "display": x.pid, "driver": os.getpid()})
    )
    time.sleep(15)
    if a.game == "bzflag":
        subprocess.run(
            ["xdotool", "key", "i"],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    end = time.monotonic() + a.seconds
    while time.monotonic() < end and client.poll() is None:
        sample = generator_sample()
        with (a.out / "generator.jsonl").open("a") as output:
            output.write(json.dumps(sample) + "\n")
        if (
            sample["memory_available"] < 1.5 * 1024**3
            or sample["memory_full_avg10"] > 5
        ):
            raise RuntimeError("load generator memory safety guard / pressure")
        # Input goes only to this client's private X server, never to the user's desktop.
        subprocess.run(
            [
                "xdotool",
                "keydown",
                "w",
                "keydown",
                "Up",
                "keydown",
                "Left",
                "mousedown",
                "1",
            ],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(3)
        subprocess.run(
            [
                "xdotool",
                "keyup",
                "w",
                "keyup",
                "Up",
                "keyup",
                "Left",
                "mouseup",
                "1",
                "key",
                "space",
            ],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(1)
    subprocess.run(
        [
            "import",
            "-display",
            env["DISPLAY"],
            "-window",
            "root",
            str(a.out / "final.png"),
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
finally:
    if x and x.poll() is None:
        try:
            subprocess.run(
                [
                    "import",
                    "-display",
                    env["DISPLAY"],
                    "-window",
                    "root",
                    str(a.out / "shutdown.png"),
                ],
                timeout=5,
                check=False,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        except subprocess.TimeoutExpired:
            pass
    for process in [client, x]:
        if process and process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()

    if private_prefix:
        try:
            subprocess.run(
                [
                    str(
                        pathlib.Path.home()
                        / ".local/share/Steam/steamapps/common/Proton - Experimental/files/bin/wineserver"
                    ),
                    "-k",
                ],
                env=env,
                timeout=15,
                check=False,
            )
        except subprocess.TimeoutExpired:
            pass
        shutil.rmtree(private_prefix, ignore_errors=True)
