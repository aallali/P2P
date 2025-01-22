import socket
import threading


def read_config():
    with open("config.txt") as f:
        host = f.readline().strip()
        port = int(f.readline().strip())
    return host, port


# Function to handle sending messages
def send_messages(sock):
    while True:
        message = input("You: ")
        sock.sendall(message.encode())


# Function to handle receiving messages
def receive_messages(sock):
    while True:
        try:
            message = sock.recv(1024).decode()
            if message:
                print(f"\nFriend: {message}")
            else:
                break
        except:
            print("\nConnection closed.")
            break


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
        conn, addr = server.accept()
        print(f"Connected to {addr}")

        # Start threads for sending and receiving
        threading.Thread(target=send_messages, args=(conn,)).start()
        threading.Thread(target=receive_messages, args=(conn,)).start()

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
