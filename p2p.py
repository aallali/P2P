# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : p2p.py                                                         #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/22 17:17:48 by aallali                                  #
#   Updated: 2025/01/24 01:33:21 by aallali                                  #
# ************************************************************************** #

import socket
import threading
import os
import argparse
import time
from src.commands import CommandHandler
from src.logger import setup_logger, log
from src.config import read_config, FILE_HEADER, END_OF_FILE
import src.config as shared_config
from src.files import receive_file


def setup_received_files_dir(received_files_dir):
    shared_config.RECEIVED_FILES_DIR = received_files_dir
    if not os.path.exists(shared_config.RECEIVED_FILES_DIR):
        os.makedirs(shared_config.RECEIVED_FILES_DIR)


def send_messages(sock):
    handler = CommandHandler(sock, shared_config.RECEIVED_FILES_DIR)
    while True:
        try:
            message = input("")
            handler.execute(message)
        except Exception as e:
            log(str(e), "SYSTEM", "ERROR")
            break


def receive_messages(sock):
    while True:
        try:
            raw_message = sock.recv(1024)
            if not raw_message:
                log("Connection closed by peer", "SYSTEM", "WARNING")
                break

            try:
                message = raw_message.decode()
                if message.startswith(FILE_HEADER):
                    _, file_name, file_size = message.split(" ", 2)
                    receive_file(sock, file_name, int(file_size))
                elif message == END_OF_FILE:
                    log("File transfer complete", "SYSTEM", "FILE")
                else:
                    log(message, "HIM", "CHAT")
            except UnicodeDecodeError:
                log("Received corrupt message", "SYSTEM", "ERROR")
                continue

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
    server_host, server_port, internal_port, received_files_dir, persistent_logging = read_config()
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
                receive_thread = threading.Thread(target=receive_messages, args=(client,))
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
