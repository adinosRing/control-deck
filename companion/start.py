from pathlib import Path
import socket
import subprocess
import sys


directory = Path(__file__).resolve().parent
python = Path(sys.executable).with_name("python.exe")
with socket.socket() as probe:
    probe.settimeout(1)
    already_listening = probe.connect_ex(("127.0.0.1", 8765)) == 0
if not already_listening:
    with (directory / "bridge.stderr.log").open("ab") as log:
        subprocess.Popen(
            [str(python), str(directory / "bridge.py")],
            cwd=directory,
            stdin=subprocess.DEVNULL,
            stdout=log,
            stderr=log,
            creationflags=subprocess.CREATE_NO_WINDOW,
        )
