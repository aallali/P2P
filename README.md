# p2p

A lightweight Python-based peer-to-peer chat system, supports file sharing.

## Usage

### Start the Server
1. Open a terminal, and run:
   ```bash
   python3 p2p.py -s
   ```

### Connect as a Client
1. Open another terminal or use another device, and run:
   ```bash
   python3 p2p.py -c
   ```

### Chat
- Type messages and press **Enter** to send.
- Messages from other users will appear in real-time.

### Commands
- `/file <path>`: Select a file to send.
- `/send`: Resend the selected file.
- `/close` or `/c`: Close the connection.
- Any other text will be sent as a chat message.

### Using ngrok
1. Download and install ngrok from [ngrok.com](https://ngrok.com/).
2. Start ngrok to tunnel TCP traffic on port `12345`:
   ```bash
   ngrok tcp 12345
   ```
3. Copy the forwarding address provided by ngrok (e.g., `tcp://0.tcp.ngrok.io:XXXXX`).
4. Send this address to the client.
    - His `.p2p.conf` file should look like:
      ```conf
      0.tcp.ngrok.io
      XXXXX
      ```
5. The client should use this address to connect.

### Example Session

#### Server
```bash
python3 p2p.py -s
```
```
[2025-01-23 13:05:25][SYSTEM][INFO] Process ID: 25446
[2025-01-23 13:05:25][SYSTEM][INFO] Server started on port 12345
[2025-01-23 13:05:44][SYSTEM][INFO] Connected to ('127.0.0.1', 49796)
[2025-01-23 13:05:44][ME][INFO] no client
[2025-01-23 13:05:44][ME][INFO] cant send
[2025-01-23 13:05:47][ME][INFO] hi
[2025-01-23 13:05:52][HIM][INFO] salam
[2025-01-23 13:06:01][HIM][INFO] i will send a file
[2025-01-23 13:06:03][ME][INFO] ok
[2025-01-23 13:06:09][HIM][INFO] Receiving: chat.txt (18 bytes)
[2025-01-23 13:06:09][SYSTEM][INFO] Receiving: 18/18 bytes (100%)
[2025-01-23 13:06:09][SYSTEM][INFO] File received: chat.txt [18 bytes]
[2025-01-23 13:06:17][HIM][INFO] received ?
[2025-01-23 13:06:22][ME][INFO] yes thanks, send again
[2025-01-23 13:06:24][HIM][INFO] Receiving: chat.txt (18 bytes)
[2025-01-23 13:06:24][SYSTEM][INFO] Receiving: 18/18 bytes (100%)
[2025-01-23 13:06:24][SYSTEM][INFO] File received: chat.txt [18 bytes]
```

#### Client
```bash
python3 p2p.py -c
```
```
[2025-01-23 13:05:43][SYSTEM][INFO] Process ID: 25568
[2025-01-23 13:05:44][SYSTEM][INFO] Connected to x.tcp.xx.ngrok.io:XXXXX
[2025-01-23 13:05:44][HIM][INFO] no client
[2025-01-23 13:05:44][HIM][INFO] cant send
[2025-01-23 13:05:47][HIM][INFO] hi
[2025-01-23 13:05:52][ME][INFO] salam
[2025-01-23 13:06:01][ME][INFO] i will send a file
[2025-01-23 13:06:03][HIM][INFO] ok
/file ../chat.txt
[2025-01-23 13:06:09][ME][INFO] Selected: ../chat.txt
[2025-01-23 13:06:09][SYSTEM][INFO] Sending: 18/18 bytes (100%)
[2025-01-23 13:06:09][ME][INFO] File sent: chat.txt [18 bytes]
[2025-01-23 13:06:17][ME][INFO] received ?
[2025-01-23 13:06:22][HIM][INFO] yes thanks, send again
/send
[2025-01-23 13:06:24][ME][INFO] Resending: ../chat.txt
[2025-01-23 13:06:24][SYSTEM][INFO] Sending: 18/18 bytes (100%)
[2025-01-23 13:06:24][ME][INFO] File sent: chat.txt [18 bytes]
```

#### Folder Structure after file exchange:
```bash
➜  p2p git:(master) ✗ tree
.
├── .p2p.conf
├── p2p.log
├── p2p.py
├── README.md
└── received_files
    └── chat.txt

2 directories, 5 files
➜  p2p git:(master) ✗
```
## Requirements
- Python 3.x

## Notes
- Use this `.p2p.conf` for local testing on your machine without needing an external device:
```
0.0.0.0
12345
```
- Use file sharing to exchange large messages.
- A `p2p.log` file is created to persist logs.