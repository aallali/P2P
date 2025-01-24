# P2P-FileBridge
A simple peer-to-peer file synchronization tool written in Go that allows real-time file synchronization between two nodes.

## Quick Start
1. Build the application:   
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
1. Add file to tracking:
    ```
    /add /path/to/file.txt
    ```
1. List files:
   ```bash
   /ls
   ```
   Output:
   ```
    Index | Watched | Size     | Path
    0     |    YES  | 1.2 MB   | /path/to/file1.txt
    1     |     NO  | 852 KB   | /path/to/file2.txt
   ```
1. Upload a file (auto-adds it to the list):
   ```bash
   /up /path/to/file.txt
   /up #0                   #upload by index
   ```
1. Watch a file (auto-adds it to the list):
   ```bash
   /w /path/to/file.txt
   ```


4. Stop watching a file:
   ```bash
    /w /path/to/file.txt    # Start watching
    /w #0                   # Watch by index
    /woff /path/to/file.txt # Stop watching
    /woff #0               # Stop by index
   ```
5. Cleanup console:
    ```
    /cl
    ```
## Notes
- Files can be referenced by path or index (#)
- Watched files auto-upload on changes
- Files are automatically added to in-memory db when uploaded for quick alias
- Use Ctrl+C to exit program

---

## Build Requirements

- Go 1.16 or later
