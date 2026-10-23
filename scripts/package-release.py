#!/usr/bin/env python3
"""Build binaries and package only the catalog files needed at runtime."""

import argparse
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument("--output", type=Path, default=Path("dist"))
args = parser.parse_args()
root = Path(__file__).resolve().parent.parent
output = args.output.resolve()
output.mkdir(parents=True, exist_ok=True)
files = [root / "README.md", *sorted((root / "docs").glob("*.md"))]
for game in sorted((root / "docker-containers").glob("*/game.json")):
    files.append(game)
    configs = (game.parent / "server.cfg", game.parent / "UnrealTournament.ini")
    files.extend(path for path in configs if path.is_file())

checksums = []
with tempfile.TemporaryDirectory(prefix="insta-game-release-") as temporary:
    binary = Path(temporary) / "insta-game"
    targets = [("linux", "amd64"), ("linux", "arm64"), ("darwin", "arm64")]
    for system, architecture in targets:
        env = dict(os.environ, CGO_ENABLED="0", GOOS=system, GOARCH=architecture)
        command = ["go", "build", "-trimpath", "-o", str(binary), "./cmd/insta-game"]
        subprocess.run(command, cwd=root, env=env, check=True)
        archive = output / f"insta-game-{system}-{architecture}.tar.gz"
        with tarfile.open(archive, "w:gz") as bundle:
            bundle.add(binary, arcname="bin/insta-game")
            for path in files:
                bundle.add(path, arcname=str(path.relative_to(root)), recursive=False)
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        checksums.append(f"{digest}  {archive.name}\n")
        print(archive)
(output / "SHA256SUMS").write_text("".join(checksums))
