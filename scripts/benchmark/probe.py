"""Read-only probes against owned benchmark game endpoints."""

import socket
import time


def probe(game, address):
    host, port = address.rsplit(":", 1)
    port = int(port)
    start = time.perf_counter()
    players = None
    try:
        if game == "bzflag":
            with socket.create_connection((host, port), timeout=1) as conn:
                conn.sendall(b"BZFLAG\r\n\r\n")
                data = conn.recv(9)
                if not data.startswith(b"BZFS"):
                    raise ValueError("invalid BZFlag greeting")
        else:
            request = {
                "fteqw": b"\x80\x00\x00\x0c\x02QUAKE\x00\x03",
                "q2repro": b"\xff" * 4 + b"getstatus",
                "ioquake3": b"\xff" * 4 + b"getstatus capacity",
                "openarena": b"\xff" * 4 + b"getstatus capacity",
                "xonotic": b"\xff" * 4 + b"getinfo",
                "ut99": b"\\status\\",
            }[game]
            with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as conn:
                conn.settimeout(1)
                conn.connect((host, port))
                conn.send(request)
                data = conn.recv(65535)
            if game == "fteqw":
                if len(data) < 5 or data[4] != 0x83:
                    raise ValueError("invalid NetQuake status")
                players = data[5:].split(b"\x00", 3)[3][0]
            elif game in ("q2repro", "ioquake3", "openarena"):
                if not data.startswith(b"\xff" * 4):
                    raise ValueError("invalid quake status")
                players = max(
                    0, len([line for line in data[4:].splitlines()[2:] if line.strip()])
                )
            elif game in ("xonotic", "ut99"):
                if game in ("xonotic",):
                    if not data.startswith(b"\xff" * 4 + b"infoResponse\n"):
                        raise ValueError("invalid DarkPlaces status")
                    payload = data.split(b"\n", 1)[1]
                else:
                    payload = data
                parts = payload.decode(errors="replace").strip("\\").split("\\")
                values = dict(zip(parts[::2], parts[1::2]))
                players = int(
                    values.get("clients" if game in ("xonotic",) else "numplayers", "0")
                )
        return {
            "ok": True,
            "latency_ms": (time.perf_counter() - start) * 1000,
            "players": players,
            "bytes": len(data),
            "error": None,
        }
    except (OSError, ValueError, IndexError) as error:
        return {
            "ok": False,
            "latency_ms": None,
            "players": None,
            "bytes": 0,
            "error": str(error),
        }


def ut_bot_count(address):
    """UT's public game-property query exposes NumBots separately from humans."""
    host, port = address.rsplit(":", 1)
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as conn:
        conn.settimeout(2)
        conn.connect((host, int(port)))
        conn.send(b"\\game_property\\NumBots\\")
        data = conn.recv(8192)
    parts = data.decode(errors="replace").strip("\\").split("\\")
    values = dict(zip(parts[::2], parts[1::2]))
    return int(values["NumBots"])
