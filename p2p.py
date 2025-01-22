import socket
import threading
import os
import signal
import sys


def read_config():
    with open("config.txt") as f:
        host = f.readline().strip()
        port = int(f.readline().strip())
    return host, port


# Watch for file changes
def watch_file(sock, file_path):
    last_modified = None
    while True:
        try:
            current_modified = os.path.getmtime(file_path)
            if last_modified is None:
                last_modified = current_modified

            if current_modified != last_modified:
                last_modified = current_modified
                print(f"File changed: {file_path}. Sending updated file...")
                send_file(sock, file_path)
        except Exception as e:
            print(f"Error watching file: {e}")
            break


# Function to handle sending messages
def send_messages(sock):
    while True:
        message = input("You: ")
        if message.startswith("/file "):
            file_path = message.split(" ", 1)[1]
            send_file(sock, file_path)
            # Start watching the file for changes
            threading.Thread(
                target=watch_file, args=(sock, file_path), daemon=True
            ).start()
        else:
            sock.sendall(message.encode())


# Function to handle receiving messages
def receive_messages(sock):
    while True:
        try:
            message = sock.recv(1024).decode()
            if message.startswith("/file "):
                file_name = message.split(" ", 1)[1]
                receive_file(sock, file_name)
            elif message:
                print(f"\nFriend: {message}")
            else:
                break
        except Exception as e:
            print(f"\nConnection closed: {e}")
            break


# Function to send a file
def send_file(sock, file_path):
    if not os.path.exists(file_path):
        print(f"File not found: {file_path}")
        return
    try:
        file_name = os.path.basename(file_path)
        sock.sendall(f"/file {file_name}".encode())

        with open(file_path, "rb") as f:
            while chunk := f.read(1024):
                sock.sendall(chunk)
        # Send an EOF marker
        sock.sendall(b"EOF")
        print(f"File sent: {file_path}")
    except Exception as e:
        print(f"Error sending file: {e}")


# Function to receive a file
def receive_file(sock, file_name):
    try:
        with open(file_name, "wb") as f:
            while True:
                chunk = sock.recv(1024)
                if b"EOF" in chunk:
                    f.write(chunk.replace(b"EOF", b""))
                    break
                f.write(chunk)
        print(f"File received: {file_name}")
    except Exception as e:
        print(f"Error receiving file: {e}")


# Handle graceful shutdown
def shutdown_server(server):
    print("\nShutting down server...")
    server.close()
    sys.exit(0)


# Main function to start the server or client
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

        # Handle Ctrl+C gracefully
        signal.signal(signal.SIGINT, lambda sig, frame: shutdown_server(server))

        while True:
            try:
                conn, addr = server.accept()
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
