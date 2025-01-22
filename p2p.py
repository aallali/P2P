import socket
import threading
import os
import signal
import sys

# Track the current file to send
current_file = None


def read_config():
    with open("config.txt") as f:
        host = f.readline().strip()
        port = int(f.readline().strip())
    return host, port


def send_messages(sock):
    global current_file
    while True:
        try:
            message = input("You: ")
            if message.startswith("/file "):
                file_path = message.split(" ", 1)[1].strip()
                if os.path.exists(file_path):
                    current_file = file_path  # Update the current file
                    print(f"File set for sending: {file_path}")
                    send_file(sock, current_file)
                else:
                    print(f"File not found: {file_path}")
            elif message == "/send":
                if current_file:
                    send_file(sock, current_file)
                else:
                    print("No file set. Use /file <path> to set a file first.")
            else:
                sock.sendall(message.strip().encode())
        except Exception as e:
            print(f"Error sending message: {e}")
            break


def receive_messages(sock):
    while True:
        try:
            message = sock.recv(1024).decode().strip()
            if message.startswith("/file "):
                # Extract file name and size from the header
                header_parts = message.split(" ", 2)
                if len(header_parts) < 3:
                    print("Invalid file header received.")
                    continue
                file_name = header_parts[1].strip()
                file_size = int(header_parts[2].strip())
                receive_file(sock, file_name, file_size)
            elif message:
                print(f"\nFriend: {message}")
            else:
                break
        except Exception as e:
            print(f"\nConnection closed: {e}")
            break


def send_file(sock, file_path):
    try:
        file_name = os.path.basename(file_path).strip()
        file_size = os.path.getsize(file_path)
        sock.sendall(f"/file {file_name} {file_size}\n".encode())

        # Add progress tracking
        sent = 0
        with open(file_path, "rb") as f:
            while chunk := f.read(1024):
                sock.sendall(chunk)
                sent += len(chunk)
                print(
                    f"\rSending: {sent}/{file_size} bytes ({int(sent/file_size*100)}%)",
                    end="",
                )
        print(f"\nFile sent: {file_path}")
    except Exception as e:
        print(f"Error sending file: {e}")


def receive_file(sock, file_name, file_size):
    try:
        received = 0
        with open(file_name, "wb") as f:
            while received < file_size:
                chunk = sock.recv(min(1024, file_size - received))
                if not chunk:
                    raise Exception("Connection closed before receiving full file")
                f.write(chunk)
                received += len(chunk)
                print(
                    f"\rReceiving: {received}/{file_size} bytes ({int(received/file_size*100)}%)",
                    end="",
                )
        print(f"\nFile received: {file_name}")
    except Exception as e:
        print(f"Error receiving file: {e}")


def shutdown_server(server, connections):
    print("\nShutting down server...")
    try:
        for conn in connections:
            conn.close()
        server.close()
        sys.exit(0)
    except Exception as e:
        print(f"Error during shutdown: {e}")
        sys.exit(1)


def main():
    choice = (
        input("Type 's' to start as server or 'c' to connect as client: ")
        .strip()
        .lower()
    )
    host, port = read_config()
    if choice == "s":
        # Server mode
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.bind(("0.0.0.0", port))
        server.listen(1)
        print("Waiting for a connection...")

        connections = []

        # Handle Ctrl+C gracefully
        def signal_handler(sig, frame):
            shutdown_server(server, connections)

        signal.signal(signal.SIGINT, signal_handler)

        while True:
            try:
                conn, addr = server.accept()
                connections.append(conn)
                print(f"Connected to {addr}")

                # Start threads for sending and receiving
                threading.Thread(target=send_messages, args=(conn,)).start()
                threading.Thread(target=receive_messages, args=(conn,)).start()
            except Exception as e:
                print(f"Error: {e}")

    elif choice == "c":
        # Client mode

        client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        client.connect((host, port))
        print("Connected to the server!")

        # Start threads for sending and receiving
        threading.Thread(target=send_messages, args=(client,)).start()
        threading.Thread(target=receive_messages, args=(client,)).start()

    else:
        print("Invalid choice. Please restart and type 's' or 'c'.")


if __name__ == "__main__":
    main()
