# p2p

This is a lightweight Python-based peer-to-peer chat system for testing real-time communication.

---

### **Usage**

1. **Run the Script**:
   - Open a terminal and start the server:
     ```bash
     python3 p2p.py
     ```
     - Type `s` to start as the server.

   - Open another terminal or run the script on another device:
     ```bash
     python3 p2p.py
     ```
     - Type `c` to connect as the client.
     - Enter the **IP address** of the server (e.g., `127.0.0.1` for local testing).

1. **Chat**:
   - Type messages in the terminal and press **Enter** to send.
   - Messages from the other user will appear in real-time.

---

### **Requirements**
- Python 3.x
- Works locally or across a network.

---

### **Notes**
- Ensure the server is running before starting the client.
- For testing on the same machine, use `127.0.0.1` as the server IP.
- Allow port `12345` through firewalls for cross-device communication.