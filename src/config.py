import os
import sys

CONFIG_FILE = ".p2p.conf"


def create_default_config():
    default_config = """server_host=0.tcp.ngrok.io
server_port=XXXXX
internal_port=12345
received_files_dir=received_files
log=True
"""
    with open(CONFIG_FILE, "w") as f:
        f.write(default_config)
    print(
        f"Default config created at {CONFIG_FILE}. Please edit it and rerun the script."
    )
    sys.exit(0)


def read_config():
    if not os.path.exists(CONFIG_FILE):
        create_default_config()

    config = {}
    with open(CONFIG_FILE) as f:
        for line in f:
            key, value = line.strip().split("=")
            # handle comments inside value
            if "#" in value:
                value = value.split("#")[0].strip()
            config[key] = value.strip()

    return (
        config["server_host"],
        int(config["server_port"]),
        int(config["internal_port"]),
        config["received_files_dir"],
        config["log"].lower() == "true",
    )
