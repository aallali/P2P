# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : files.py                                                       #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/24 00:43:56 by aallali                                  #
#   Updated: 2025/01/24 01:33:59 by aallali                                  #
# ************************************************************************** #

import os
from .logger import log
from src.config import FILE_HEADER, END_OF_FILE
import src.config as shared_config
import traceback


def send_file(sock, file_path):
    try:
        file_name = os.path.basename(file_path)
        file_size = os.path.getsize(file_path)
        sock.sendall(f"{FILE_HEADER} {file_name} {file_size}".encode())
        sent = 0
        with open(file_path, "rb") as f:
            while chunk := f.read(1024):
                sock.sendall(chunk)
                sent += len(chunk)
                bytes_sent = f"{sent}/{file_size} bytes"
                bytes_percent = int(sent / file_size * 100)
                if sent % (file_size // 10) == 0:
                    log(f"Sending: {bytes_sent} ({bytes_percent}%)", "SYSTEM", "FILE")
        sock.sendall(END_OF_FILE.encode())
        log(f"File sent: {file_name} [{file_size} bytes]", "ME", "FILE")
    except Exception as e:
        log(f"Error sending file: {e}", "SYSTEM", "ERROR")
        traceback.print_exc()


def receive_file(sock, file_name, file_size):
    try:
        received = 0
        file_path = os.path.join(shared_config.RECEIVED_FILES_DIR, file_name)
        with open(file_path, "wb") as f:
            while received < file_size:
                chunk = sock.recv(min(1024, file_size - received))
                if not chunk:
                    break
                f.write(chunk)
                received += len(chunk)
                bytes_received = f"{received}/{file_size} bytes"
                bytes_percent = int(received / file_size * 100)
                if received % (file_size // 10) == 0:
                    log(
                        f"Receiving: {bytes_received} ({bytes_percent}%)",
                        "SYSTEM",
                        "FILE",
                    )
        log(f"File received: {file_name} [{file_size} bytes]", "SYSTEM", "FILE")
    except Exception as e:
        log(f"Error receiving file: {e}", "SYSTEM", "FILE")
