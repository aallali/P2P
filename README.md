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
- Host listens for incoming connections
- Peer connects to the host
- Both sides monitor their shared folders for changes
- Changes are automatically synchronized between nodes
- Periodic ping/pong ensures connection health

## Build Requirements

- Go 1.16 or later
