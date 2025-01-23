# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : p2p.py                                                         #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/22 17:17:48 by aallali                                  #
#   Updated: 2025/01/23 17:14:00 by aallali                                  #
# ************************************************************************** #

import socket
import threading
import os
import sys
import argparse
import time
from src.logger import setup_logger, log
from src.config import read_config

RECEIVED_FILES_DIR = "received_files"
# Track the current file to send
current_file = None


def close_socket(sock):
    try:
        sock.settimeout(2)
        sock.shutdown(socket.SHUT_RDWR)
        sock.close()
    except socket.error:
        pass
    finally:
        sys.exit(0)


def send_file(sock, file_path):
    try:
        file_name = os.path.basename(file_path)
        file_size = os.path.getsize(file_path)
        sock.sendall(f"/file {file_name} {file_size}".encode())

        sent = 0
        with open(file_path, "rb") as f:
            while chunk := f.read(1024):
                sock.sendall(chunk)
                sent += len(chunk)
                bytes_sent = f"{sent}/{file_size} bytes"
                bytes_percent = int(sent / file_size * 100)
                log(f"Sending: {bytes_sent} ({bytes_percent}%)", "SYSTEM", "FILE")
        log(f"File sent: {file_name} [{file_size} btyes]", "ME", "FILE")
    except Exception as e:
        log(f"Error sending file: {e}", "SYSTEM", "ERROR")


def setup_received_files_dir(received_files_dir):
    global RECEIVED_FILES_DIR
    RECEIVED_FILES_DIR = received_files_dir
    if not os.path.exists(RECEIVED_FILES_DIR):
        os.makedirs(RECEIVED_FILES_DIR)


def receive_file(sock, file_name, file_size):
    try:
        received = 0
        file_path = os.path.join(RECEIVED_FILES_DIR, file_name)
        with open(file_path, "wb") as f:
            while received < file_size:
                chunk = sock.recv(min(1024, file_size - received))
                if not chunk:
                    break
                f.write(chunk)
                received += len(chunk)
                bytes_received = f"{received}/{file_size} bytes"
                bytes_percent = int(received / file_size * 100)
                log(
                    f"Receiving: {bytes_received} ({bytes_percent}%)",
                    "SYSTEM",
                    "FILE",
                )
        log(f"File received: {file_name} [{file_size} bytes]", "SYSTEM", "FILE")
    except Exception as e:
        log(f"Error receiving file: {e}", "SYSTEM", "FILE")


def send_messages(sock):
    global current_file
    while True:
        try:
            message = input("")
            if message.startswith("/file "):
                file_path = message.split(" ", 1)[1].strip()
                if os.path.exists(file_path):
                    current_file = file_path
                    log(f"Selected: {file_path}", "ME", "FILE")
                    send_file(sock, current_file)
                else:
                    log(f"File not found: {file_path}", "SYSTEM", "ERROR")
            elif message == "/send":
                if current_file and os.path.exists(current_file):
                    log(f"Resending: {current_file}", "ME", "FILE")
                    send_file(sock, current_file)
                else:
                    log("No file selected", "SYSTEM", "ERROR")
            elif message in ["/close", "/c"]:
                log("Closing connection", "SYSTEM", "INFO")
                close_socket(sock)
            else:
                log(message, "ME", "CHAT")
                sock.sendall(message.encode())
        except Exception as e:
            log(str(e), "SYSTEM", "ERROR")
            break


def receive_messages(sock):
    while True:
        try:
            message = sock.recv(1024).decode()
            if not message:
                log("Connection closed by peer", "SYSTEM", "WARNING")
                break
            if message.startswith("/file "):
                _, file_name, file_size = message.strip().split(" ")
                log(f"Receiving: {file_name} ({file_size} bytes)", "HIM", "FILE")
                receive_file(sock, file_name, int(file_size))
            else:
                log(message, "HIM", "CHAT")
        except Exception as e:
            log(str(e), "SYSTEM", "ERROR")
            break


def handle_client(conn, addr):
    log(f"Connected to {addr}", role="SYSTEM", msg_type="INFO")
    send_thread = threading.Thread(target=send_messages, args=(conn,))
    receive_thread = threading.Thread(target=receive_messages, args=(conn,))
    send_thread.start()
    receive_thread.start()
    send_thread.join()
    receive_thread.join()
    log(f"Connection closed: {addr}", role="SYSTEM", msg_type="INFO")


def main():
    server_host, server_port, internal_port, received_files_dir, persistent_logging = (
        read_config()
    )
    setup_logger(persistent_logging)
    setup_received_files_dir(received_files_dir)
    log(f"Process ID: {os.getpid()}", role="SYSTEM", msg_type="INFO")
    parser = argparse.ArgumentParser(description="P2P Chat Application")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("-s", "--server", action="store_true", help="Start as server")
    group.add_argument("-c", "--client", action="store_true", help="Start as client")
    args = parser.parse_args()

    if args.server:
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(("0.0.0.0", internal_port))
        server.listen(5)  # Allow up to 5 pending connections
        log(f"Server started on port {internal_port}", role="SYSTEM", msg_type="INFO")

        while True:
            conn, addr = server.accept()
            client_thread = threading.Thread(target=handle_client, args=(conn, addr))
            client_thread.start()

    elif args.client:
        while True:
            try:
                client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                client.connect((server_host, server_port))
                log(
                    f"Connected to {server_host}:{server_port}",
                    role="SYSTEM",
                    msg_type="INFO",
                )
                send_thread = threading.Thread(target=send_messages, args=(client,))
                receive_thread = threading.Thread(
                    target=receive_messages, args=(client,)
                )
                send_thread.start()
                receive_thread.start()
                send_thread.join()
                receive_thread.join()
            except Exception as e:
                log(
                    f"Connection error: {e}. Retrying...",
                    role="SYSTEM",
                    msg_type="ERROR",
                )
                time.sleep(5)  # Wait before retrying


if __name__ == "__main__":
    main()
