# P2P-FileBridge
A simple peer-to-peer file synchronization tool written in Go that allows real-time file synchronization between two nodes.

## Features
- Real-time file synchronization
- Supports both host and peer modes
- Automatic reconnection for peers
- File change detection
- Simple JSON configuration

## Quick Start1. 
Build the application:   
```bash
go build   
```
2. Run the application first time to generate config:   
    ```bash   
    ./p2p   
    ```
3. Edit `config.json`:   
- For host mode:     
    ```json     
    {       
        "mode": "host",       
        "ip": "0.0.0.0",       
        "port": 12345,       
        "folder": "./shared"     
    }     
    ```
- For peer mode:     
    ```json    
    {       
        "mode": "peer",       
        "ip": "<host-ip>",       
        "port": 12345,       
        "folder": "./shared"     
    }     
    ```
4. Run the application again after configuring:   
```bash
./p2p   
```
## How It Works
1. Upload a file (auto-adds it to the list):
   ```bash
   /up /path/to/file.txt
   ```
2. Watch a file (auto-adds it to the list):
   ```bash
   /w /path/to/file.txt
   ```
3. List files:
   ```bash
   /ls
   ```
   Output:
   ```
   Index | Watched | Size | Path
      0 |     YES | 1024 | /path/to/file.txt
   ```

4. Stop watching a file:
   ```bash
   /woff #0
   ```

---

## Build Requirements

- Go 1.16 or later
