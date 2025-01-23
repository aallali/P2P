import socket
import threading
import os
import sys

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
                    f"\rSending: {sent}/{file_size} bytes ({int(sent/file_size*100)}%)",
                    end="",
                )
        print(f"\nFile sent: {file_name}")
    except Exception as e:
        print(f"Error sending file: {e}")


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
            message = input("You: ")
            if message.startswith("/file "):
                file_path = message.split(" ", 1)[1].strip()
                if os.path.exists(file_path):
                    current_file = file_path
                    print(f"Current file set to: {file_path}")
                    send_file(sock, current_file)
                else:
                    print(f"File not found: {file_path}")
            elif message == "/send":
                if current_file and os.path.exists(current_file):
                    print(f"Resending file: {current_file}")
                    send_file(sock, current_file)
                else:
                    print("No file selected. Use /file first")
            elif message in ["/close", "/c"]:
                print("Closing connection...")
                close_socket(sock)
            else:
                sock.sendall(message.encode())
        except Exception as e:
            print(f"Error: {e}")
            break


def receive_messages(sock):
    while True:
        try:
            message = sock.recv(1024).decode()
            if not message:
                print("\nConnection closed by peer")
                break

            if message.startswith("/file "):
                _, file_name, file_size = message.strip().split(" ")
                receive_file(sock, file_name, int(file_size))
            else:
                print(f"\nFriend: {message}")
        except Exception as e:
            print(f"\nError: {e}")
            break


def main():
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
