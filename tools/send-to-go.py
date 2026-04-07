# /// script
# requires-python = ">=3.9"
# dependencies = ["protobuf>=4.25.1", "ulid-py>=1.1.0"]
# ///
"""Connect directly to a Go SAPIENT listener and send Dstl reference messages.

Usage: uv run tools/send-to-go.py <host> <port>
"""
import socket
import struct
import sys
from pathlib import Path

for candidate in [
    Path(__file__).parent.parent / ".." / "dstl" / "Apex-SAPIENT-Middleware",
]:
    if (candidate / "sapient_msg").exists():
        sys.path.insert(0, str(candidate.resolve()))
        sys.path.insert(0, str((candidate / "tests").resolve()))
        break
else:
    print("ERROR: Cannot find Apex-SAPIENT-Middleware", file=sys.stderr)
    sys.exit(1)

import ulid
from msg_templates import (
    get_register_template,
    get_status_message_template,
    get_detection_message_template,
    get_alert_message_template,
    json_to_proto,
)
from sapient_msg.latest.sapient_message_pb2 import SapientMessage

HOST = sys.argv[1] if len(sys.argv) > 1 else "localhost"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 5020
NODE_ID = "b7654cdf-4328-47de-81fa-c495589e30c9"


def new_ulid():
    return str(ulid.new())


def send(sock, msg_dict):
    msg = json_to_proto(SapientMessage, msg_dict)
    data = msg.SerializeToString()
    sock.sendall(struct.pack("<I", len(data)) + data)


def main():
    sock = socket.create_connection((HOST, PORT), timeout=5)

    send(sock, get_register_template(NODE_ID))
    send(sock, get_status_message_template(NODE_ID, new_ulid()))
    send(sock, get_detection_message_template(NODE_ID, new_ulid(), new_ulid()))
    send(sock, get_alert_message_template(NODE_ID, new_ulid()))

    sock.close()


if __name__ == "__main__":
    main()
