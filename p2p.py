import socket
import threading
import os
import sys
from datetime import datetime
import logging
import argparse
import time


# ANSI colors for terminal
class Colors:
    ME = "\033[96m"  # Cyan for my messages
    HIM = "\033[95m"  # Purple for their messages
    INFO = "\033[94m"  # Blue for system info
    SUCCESS = "\033[92m"  # Green for success
    WARNING = "\033[93m"  # Yellow for warnings
    ERROR = "\033[91m"  # Red for errors
    RESET = "\033[0m"


def setup_logger():
    class P2PFormatter(logging.Formatter):
        def format(self, record):
            timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

            # Determine role and assign color
            if hasattr(record, "role"):
                if record.role == "ME":
                    color = Colors.ME
                elif record.role == "HIM":
                    color = Colors.HIM
                elif record.role == "SYSTEM":
                    color = Colors.INFO
                else:
                    color = Colors.RESET
            else:
                color = Colors.RESET

            # Determine message type color
            if record.levelname == "ERROR":
                msg_color = Colors.ERROR
            elif record.levelname == "WARNING":
                msg_color = Colors.WARNING
            elif record.levelname == "SUCCESS":
                msg_color = Colors.SUCCESS
            else:
                msg_color = color  # Default to role color

            # Format the message with colors
            formatted_message = f"{color}[{timestamp}][{record.role}][{record.levelname}] {msg_color}{record.getMessage()}{Colors.RESET}"
            return formatted_message

    logger = logging.getLogger("p2p")
    logger.setLevel(logging.DEBUG)  # Set to DEBUG to capture all levels

    # Console handler with colored output
    ch = logging.StreamHandler()
    ch.setLevel(logging.DEBUG)
    ch.setFormatter(P2PFormatter())
    logger.addHandler(ch)

    # File handler without colors
    fh = logging.FileHandler("p2p.log")
    fh.setLevel(logging.DEBUG)
    fh.setFormatter(
        logging.Formatter("[%(asctime)s][%(role)s][%(levelname)s] %(message)s")
    )
    logger.addHandler(fh)

    return logger


def log(msg, role="SYSTEM", msg_type="INFO"):
    logger = logging.getLogger("p2p")
    level = logging.ERROR if msg_type == "ERROR" else logging.INFO
    logger.log(level, msg, extra={"role": role, "msg_type": msg_type})


# Track the current file to send
current_file = None


def read_config():
    config_paths = [
        ".p2p.conf",  # Hidden local config
        os.path.expanduser("~/.p2p.conf"),  # Hidden user config
    ]

    for path in config_paths:
        if os.path.exists(path):
            with open(path) as f:
                host = f.readline().strip()
                port = int(f.readline().strip())
            return host, port

    # Default fallback
    return "127.0.0.1", 12345


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
                print(f"\r[Sending: {bytes_sent} ({bytes_percent}%)]", end="")
        log(f"\nFile sent: {file_name} [{file_size} btyes]", "ME", "FILE")
    except Exception as e:
        log(f"Error sending file: {e}", "SYSTEM", "ERROR")


def receive_file(sock, file_name, file_size):
    try:
        received = 0
        with open(file_name, "wb") as f:
            while received < file_size:
                chunk = sock.recv(min(1024, file_size - received))
                if not chunk:
                    break
                f.write(chunk)
                received += len(chunk)
                bytes_received = f"{received}/{file_size} bytes"
                bytes_percent = int(received / file_size * 100)
                print(f"\r[Receiving: {bytes_received} ({bytes_percent}%)]", end="")
        log(f"\nFile received: {file_name} [{file_size} btyes]", "SYSTEM", "FILE")
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
    setup_logger()
    log(f"Process ID: {os.getpid()}", role="SYSTEM", msg_type="INFO")
    parser = argparse.ArgumentParser(description="P2P Chat Application")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("-s", "--server", action="store_true", help="Start as server")
    group.add_argument("-c", "--client", action="store_true", help="Start as client")
    args = parser.parse_args()
    host, port = read_config()

    if args.server:
        port = 12345
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(("0.0.0.0", port))
        server.listen(5)  # Allow up to 5 pending connections
        log(f"Server started on port {port}", role="SYSTEM", msg_type="INFO")

        while True:
            conn, addr = server.accept()
            client_thread = threading.Thread(target=handle_client, args=(conn, addr))
            client_thread.start()

    elif args.client:
        while True:
            try:
                client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                client.connect((host, port))
                log(f"Connected to {host}:{port}", role="SYSTEM", msg_type="INFO")
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
