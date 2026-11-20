"""Generate this PC's private credentials and the two files copied to Kindle."""

import argparse
import ipaddress
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile

COMPANION = Path(__file__).resolve().parent
PRIVATE_NETWORKS = tuple(
    ipaddress.ip_network(block) for block in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")
)


def local_address(value):
    try:
        address = ipaddress.IPv4Address(value)
    except ValueError as error:
        raise argparse.ArgumentTypeError("Enter the PC's local IPv4 address.") from error
    if not any(address in network for network in PRIVATE_NETWORKS):
        raise argparse.ArgumentTypeError("Use the PC's private IPv4 address on your LAN.")
    return address


def local_network(value):
    try:
        network = ipaddress.IPv4Network(value, strict=False)
    except ValueError as error:
        raise argparse.ArgumentTypeError("Enter an IPv4 network in CIDR notation.") from error
    if not any(network.subnet_of(block) for block in PRIVATE_NETWORKS):
        raise argparse.ArgumentTypeError("The address range must be a private IPv4 network.")
    return network


def write_local(path, text):
    # Private by default on POSIX, and inherits the user's directory ACL on Windows.
    fd, name = tempfile.mkstemp(prefix=".pair-", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            stream.write(text)
        os.replace(name, path)
    finally:
        Path(name).unlink(missing_ok=True)


def ensure_identity(key, cert, openssl):
    if key.exists() != cert.exists():
        raise SystemExit("The existing TLS key and certificate are incomplete. Restore the missing file before pairing.")
    if not key.exists():
        with tempfile.TemporaryDirectory(dir=key.parent) as folder:
            staged_key = Path(folder) / key.name
            staged_cert = Path(folder) / cert.name
            command = [
                openssl, "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1095",
                "-subj", "/CN=kindle-dashboard.local",
                "-addext", "subjectAltName=DNS:kindle-dashboard.local",
                "-addext", "extendedKeyUsage=serverAuth",
                "-out", str(staged_cert), "-keyout", str(staged_key),
            ]
            subprocess.run(command, check=True, capture_output=True)
            staged_key.replace(key)
            staged_cert.replace(cert)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pc-address", action="append", required=True, type=local_address)
    parser.add_argument("--allow-network", action="append", type=local_network, default=[],
                        help="Kindle's LAN in CIDR notation. Defaults to each PC address's /24.")
    parser.add_argument("--output", type=Path,
                        default=COMPANION.parent / "dist" / "KindleDashboardNative")
    parser.add_argument("--openssl", default=shutil.which("openssl"),
                        help="Path to the OpenSSL executable.")
    args = parser.parse_args()
    if not args.openssl:
        parser.error("OpenSSL is required. Pass the path to openssl.exe with --openssl.")

    args.output.mkdir(parents=True, exist_ok=True)
    key, cert = (COMPANION / name for name in ("bridge-key.pem", "bridge-cert.pem"))
    ensure_identity(key, cert, args.openssl)
    pc_config = COMPANION / "bridge-config.json"
    settings = json.loads(pc_config.read_text(encoding="utf-8")) if pc_config.exists() else {}
    settings.setdefault("token", secrets.token_urlsafe(40))
    settings.setdefault("browser_token", secrets.token_urlsafe(40))
    networks = args.allow_network or [
        ipaddress.IPv4Network((address, 24), strict=False) for address in args.pc_address
    ]
    settings.update(port=8765, networks=["127.0.0.0/8", *dict.fromkeys(map(str, networks))])
    write_local(pc_config, json.dumps(settings, indent=2) + "\n")
    write_local(COMPANION / "youtube-extension" / "pairing.js",
                "const browserPairingToken = " + json.dumps(settings["browser_token"]) + ";\n")

    kindle = {"addresses": list(dict.fromkeys(map(str, args.pc_address))),
              "port": settings["port"], "token": settings["token"]}
    write_local(args.output / "pc-bridge.json", json.dumps(kindle, indent=2) + "\n")
    shutil.copyfile(cert, args.output / "pc-bridge.pem")
    print(f"Kindle pairing files created in {args.output}.")


if __name__ == "__main__":
    main()
