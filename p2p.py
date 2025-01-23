import socket
import threading
import os
import sys
from datetime import datetime
import logging


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

            # Get message color based on role
            color = (
                Colors.ME
                if record.role == "ME"
                else (
                    Colors.HIM
                    if record.role == "HIM"
                    else Colors.ERROR if record.levelname == "ERROR" else Colors.INFO
                )
            )

            # Format: [time][role][type] colored_message
            return f"[{timestamp}][{record.role}][{record.msg_type}] {color}{record.msg}{Colors.RESET}"

    # Create logger
    logger = logging.getLogger("p2p")
    logger.setLevel(logging.INFO)

    # File handler - no colors
    fh = logging.FileHandler("p2p.log")
    fh.setFormatter(
        logging.Formatter("[%(asctime)s][%(role)s][%(msg_type)s] %(message)s")
    )

    # Console handler - with colors
    ch = logging.StreamHandler()
    ch.setFormatter(P2PFormatter())

    logger.addHandler(fh)
    logger.addHandler(ch)
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
                print(
                    f"\n[Sending: {sent}/{file_size} bytes ({int(sent/file_size*100)}%)]"
                )
        log(f"File sent: {file_name}", "ME", "FILE")
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
                print(
                    f"\rReceiving: {received}/{file_size} bytes ({int(received/file_size*100)}%)",
                    end="",
                )
        print(f"\nFile received: {file_name}")
    except Exception as e:
        print(f"Error receiving file: {e}")


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


def main():
    logger = setup_logger()
    print(f"Process ID: {os.getpid()}")
    choice = (
        input("Type 's' to start as server or 'c' to connect as client: ")
        .strip()
        .lower()
    )
    host, port = read_config()

    if choice == "s":
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(("0.0.0.0", port))
        server.listen(1)
        print(f"Server started on port {port}")
        conn, addr = server.accept()
        print(f"Connected to {addr}")

        send_thread = threading.Thread(target=send_messages, args=(conn,))
        receive_thread = threading.Thread(target=receive_messages, args=(conn,))
        send_thread.start()
        receive_thread.start()
        send_thread.join()

    elif choice == "c":
        client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        client.connect((host, port))
        print(f"Connected to {host}:{port}")

        send_thread = threading.Thread(target=send_messages, args=(client,))
        receive_thread = threading.Thread(target=receive_messages, args=(client,))
        send_thread.start()
        receive_thread.start()
        send_thread.join()


if __name__ == "__main__":
    main()
