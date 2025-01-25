// ************************************************************************** //
//   Copyright © hi@allali.me                                                 //
//                                                                            //
//   File    : main.go                                                        //
//   Project : p2p                                                            //
//   License : MIT                                                            //
//                                                                            //
//   Created: 2025/01/24 17:27:43 by aallali                                  //
//   Updated: 2025/01/25 15:47:58 by aallali                                  //
// ************************************************************************** //

package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	ConfigFile = "config.json"
	ChunkSize  = 1024 * 1024 // 1MB chunks
)

// Config structure
type Config struct {
	Mode        string `json:"mode"` // "host" or "peer"
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Folder      string `json:"folder"`
	Password    string `json:"password"`
	WhitelistIP string `json:"peer_ip"` // Added whitelist IP field
}

// Message structure
type Message struct {
	Action    string `json:"action"`    // "upload", "notification"
	Path      string `json:"path"`      // File path
	Content   string `json:"content"`   // File content (base64 encoded)
	TotalSize int64  `json:"totalSize"` // Total file size
}

// Add new message type for authentication
type AuthMessage struct {
	Password string `json:"password"`
	Status   string `json:"status"` // "ok" or "failed"
}

// FileEntry represents a file in memory
type FileEntry struct {
	Path    string // Full path of the file
	Size    int64  // Size of the file
	Watched bool   // Whether the file is being watched
}

// FileManager manages the list of files
type FileManager struct {
	Files []FileEntry
	Mutex sync.Mutex
}

// Add new type for connection state management
type ConnectionState struct {
	isConnected bool
	mutex       sync.Mutex
}

func (cs *ConnectionState) setConnected(connected bool) {
	cs.mutex.Lock()
	cs.isConnected = connected
	cs.mutex.Unlock()
}

func (cs *ConnectionState) isActive() bool {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.isConnected
}

// Add global connection state
var connState = ConnectionState{}

// Add new types and globals for IP jailing
type IPJail struct {
	attempts map[string]int
	jailed   map[string]time.Time
	mutex    sync.RWMutex
}

const (
	MaxAttempts = 5
	JailTime    = 5 * time.Minute
)

var ipJail = IPJail{
	attempts: make(map[string]int),
	jailed:   make(map[string]time.Time),
}

// CurrentConn holds the active network connection
var CurrentConn net.Conn

// ConnMutex guards access to CurrentConn
var ConnMutex sync.Mutex

// Add methods for IP jailing
func (j *IPJail) incrementAttempt(ip string) int {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.attempts[ip]++
	if j.attempts[ip] >= MaxAttempts {
		j.jailed[ip] = time.Now().Add(JailTime)
		delete(j.attempts, ip)
	}
	return j.attempts[ip]
}

func (j *IPJail) isJailed(ip string) bool {
	j.mutex.RLock()
	defer j.mutex.RUnlock()
	if jailTime, exists := j.jailed[ip]; exists {
		if time.Now().After(jailTime) {
			// Auto-release from jail if time has expired
			j.mutex.Lock()
			delete(j.jailed, ip)
			j.mutex.Unlock()
			return false
		}
		return true
	}
	return false
}

// Add new type for file assembly
type FileAssembly struct {
	TotalSize    int64
	ReceivedSize int64
	TempFile     *os.File
}

// Add map to track file assemblies
var (
	fileAssemblies = make(map[string]*FileAssembly)
	assemblyMutex  sync.Mutex
)

func loadConfig() Config {
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		defaultConfig := Config{
			Mode:        "host",
			IP:          "0.0.0.0",
			Port:        12345,
			Folder:      "./shared",
			Password:    "1337",
			WhitelistIP: "", // Empty means accept any IP
		}
		configData, _ := json.MarshalIndent(defaultConfig, "", "  ")
		os.WriteFile(ConfigFile, configData, 0644)
		fmt.Printf("Config file created. Edit '%s' and rerun.\n", ConfigFile)
		os.Exit(0)
	}

	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		panic(err)
	}

	return config
}

// Add helper function to validate IP
func isIPAllowed(config Config, remoteAddr string) bool {
	if config.WhitelistIP == "" {
		return true // Accept any IP if whitelist is empty
	}

	// Extract IP from remoteAddr (removes port)
	clientIP := strings.Split(remoteAddr, ":")[0]
	return clientIP == config.WhitelistIP
}

func authenticateConnection(expectedPassword string) bool {
	// Set a timeout for authentication
	CurrentConn.SetDeadline(time.Now().Add(10 * time.Second))
	defer CurrentConn.SetDeadline(time.Time{})

	var authMessage AuthMessage
	decoder := json.NewDecoder(CurrentConn)
	if err := decoder.Decode(&authMessage); err != nil {
		return false
	}

	// Send authentication response
	response := AuthMessage{Status: "failed"}
	if authMessage.Password == expectedPassword {
		response.Status = "ok"
	}
	encoder := json.NewEncoder(CurrentConn)
	encoder.Encode(response)

	return authMessage.Password == expectedPassword
}

func startHost(config Config) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	logMessage("Hosting on %s:%d. Waiting for connection...\n", config.IP, config.Port)

	for {
		CurrentConn, err = listener.Accept()
		if err != nil {
			logMessage("Error accepting connection: %v\n", err)
			continue
		}

		logMessage("Connection state: %v\n", connState.isActive())
		if connState.isActive() {
			// reject with msg if peer is already connected
			logMessage("Peer already connected. Rejecting new connection...\n")
			// send rejection msg to that connection
			rejectionMessage := Message{Action: "notification", Content: "Peer already connected. Try again later."}
			encoder := json.NewEncoder(CurrentConn)
			encoder.Encode(rejectionMessage)
			CurrentConn.Close()
			continue
		}

		// Extract IP from remote address
		remoteAddr := CurrentConn.RemoteAddr().String()
		clientIP := strings.Split(remoteAddr, ":")[0]

		// Check if IP is jailed
		if ipJail.isJailed(clientIP) {
			// logMessage("Connection rejected: IP %s is temporarily blocked\n", clientIP)
			CurrentConn.Close()
			continue
		}

		// Check if IP is allowed
		if !isIPAllowed(config, remoteAddr) {
			attempts := ipJail.incrementAttempt(clientIP)
			remaining := MaxAttempts - attempts
			if remaining > 0 {
				logMessage("Connection rejected from non-whitelisted IP: %s (%d attempts remaining)\n",
					clientIP, remaining)
			} else {
				logMessage("IP %s has been temporarily blocked for %v\n",
					clientIP, JailTime)
			}
			CurrentConn.Close()
			continue
		}

		// Authenticate the connection
		if !authenticateConnection(config.Password) {
			attempts := ipJail.incrementAttempt(clientIP)
			remaining := MaxAttempts - attempts
			if remaining > 0 {
				logMessage("Authentication failed from %s (%d attempts remaining)\n",
					clientIP, remaining)
			} else {
				logMessage("IP %s has been temporarily blocked for %v\n",
					clientIP, JailTime)
			}
			CurrentConn.Close()
			continue
		}

		// Reset attempts on successful authentication
		ipJail.mutex.Lock()
		delete(ipJail.attempts, clientIP)
		ipJail.mutex.Unlock()

		logMessage("Welcome Peer IP: %s\n", CurrentConn.RemoteAddr().String())
		connState.setConnected(true)
		// Handle the connection in a new goroutine
		go handleConnection(config)
	}
}

func connectToHost(config Config) {
	for {
		if connState.isActive() {
			time.Sleep(1 * time.Second)
			continue
		}

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
		if err != nil {
			logMessage("Host not available. Retrying in 3 seconds...\n")
			time.Sleep(3 * time.Second)
			continue
		}

		ConnMutex.Lock()
		CurrentConn = conn
		ConnMutex.Unlock()

		// Set connected state
		connState.setConnected(true)

		// Send authentication message
		authMessage := AuthMessage{Password: config.Password}
		encoder := json.NewEncoder(conn)
		if err := encoder.Encode(authMessage); err != nil {
			logMessage("Failed to send authentication: %v\n", err)
			ConnMutex.Lock()
			CurrentConn.Close()
			CurrentConn = nil
			ConnMutex.Unlock()
			connState.setConnected(false)
			continue
		}

		// Wait for authentication response
		var response AuthMessage
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&response); err != nil {
			logMessage("Failed to receive authentication response: %v\n", err)
			ConnMutex.Lock()
			CurrentConn.Close()
			CurrentConn = nil
			ConnMutex.Unlock()
			connState.setConnected(false)
			time.Sleep(5 * time.Second)
			continue
		}

		if response.Status != "ok" {
			logMessage("Authentication failed: Invalid password or peer is unavailabe\n")
			// quit program if invalid password

			conn.Close()
			connState.setConnected(false)
			os.Exit(1)
		}

		logMessage("Connected and authenticated to host.\n")
		handleConnection(config)

		// Reset connection state after disconnection
		connState.setConnected(false)
		time.Sleep(1 * time.Second) // Add delay before reconnection attempt
	}
}

var fileManager = FileManager{}

func handleConnection(config Config) {
	defer func() {
		logMessage("Peer disconnected.[0]\n")
		CurrentConn.Close()
		connState.setConnected(false)
		ConnMutex.Lock()
		CurrentConn = nil
		ConnMutex.Unlock()
	}()

	quit := make(chan bool)

	sendMessage(Message{Action: "notification", Content: "Connected!"})

	receivedFiles := make(map[string]bool)
	var receivedFilesMutex sync.Mutex

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logMessage("Error creating watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	go func() {
		reader := bufio.NewReader(CurrentConn)
		for {
			select {
			case <-quit:
				logMessage("Quit go routine 1\n")
				return
			default:
				message, err := readMessage(reader)
				if err != nil {
					if err == io.EOF {
						logMessage("Peer disconnected.[1]\n")
						quit <- true

						return
					} else {
						logMessage("Error reading message: %v\n", err)
					}
					return
				}

				switch message.Action {
				case "upload":
					filePath := filepath.Join(config.Folder, message.Path)
					os.MkdirAll(filepath.Dir(filePath), 0755)

					content, err := base64.StdEncoding.DecodeString(message.Content)
					if err != nil {
						logMessage("Error decoding content: %v\n", err)
						continue
					}

					assemblyMutex.Lock()
					assembly, exists := fileAssemblies[filePath]
					if !exists {
						tempFile, err := os.CreateTemp("", "upload-*")
						if err != nil {
							assemblyMutex.Unlock()
							logMessage("Error creating temp file: %v\n", err)
							continue
						}
						assembly = &FileAssembly{
							TotalSize: message.TotalSize,
							TempFile:  tempFile,
						}
						fileAssemblies[filePath] = assembly
					}
					assemblyMutex.Unlock()

					if _, err := assembly.TempFile.Write(content); err != nil {
						logMessage("Error writing chunk: %v\n", err)
						continue
					}

					assembly.ReceivedSize += int64(len(content))
					logMessage("\rDownloading %s: %d/%d bytes (%d%%)",
						message.Path,
						assembly.ReceivedSize,
						assembly.TotalSize,
						(assembly.ReceivedSize*100)/assembly.TotalSize,
					)

					if assembly.ReceivedSize >= assembly.TotalSize {
						assembly.TempFile.Close()
						if err := os.Rename(assembly.TempFile.Name(), filePath); err != nil {
							logMessage("\nError saving file: %v\n", err)
							os.Remove(assembly.TempFile.Name())
						} else {
							fmt.Println()
							logMessage("File saved: %s\n", filePath)
						}

						assemblyMutex.Lock()
						delete(fileAssemblies, filePath)
						assemblyMutex.Unlock()

						receivedFilesMutex.Lock()
						receivedFiles[filePath] = true
						receivedFilesMutex.Unlock()

						go func() {
							time.Sleep(2 * time.Second)
							receivedFilesMutex.Lock()
							delete(receivedFiles, filePath)
							receivedFilesMutex.Unlock()
						}()
					}

				case "notification":
					logMessage("Notification from peer: %s\n", message.Content)
				}
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			select {
			case <-quit:
				return
			default:
				command := scanner.Text()
				parts := strings.Fields(command)
				if len(parts) == 0 {
					continue
				}

				switch parts[0] {
				case "/up":
					if len(parts) < 2 {
						logMessage("Usage: /up <file> or /up #<number>\n")
						continue
					}
					filePath := parts[1]
					if strings.HasPrefix(filePath, "#") {
						index := parseIndex(filePath)
						if index == -1 {
							logMessage("Invalid index.\n")
							continue
						}
						fileManager.Mutex.Lock()
						if index >= len(fileManager.Files) {
							logMessage("Index out of range.\n")
							fileManager.Mutex.Unlock()
							continue
						}
						filePath = fileManager.Files[index].Path
						fileManager.Mutex.Unlock()
					} else {
						fileManager.Mutex.Lock()
						if !fileManager.contains(filePath) {
							fileInfo, err := os.Stat(filePath)
							if err != nil {
								logMessage("Error accessing file: %v\n", err)
								fileManager.Mutex.Unlock()
								continue
							}
							fileManager.Files = append(fileManager.Files, FileEntry{
								Path:    filePath,
								Size:    fileInfo.Size(),
								Watched: false,
							})
							logMessage("Added file: %s\n", filePath)
						}
						fileManager.Mutex.Unlock()
					}
					if err := sendFileWithProgress(filePath); err != nil {
						logMessage("Error uploading file: %v\n", err)
					} else {
						logMessage("File uploaded successfully!\n")
					}

				case "/w":
					if len(parts) < 2 {
						logMessage("Usage: /w <file> or /w #<number>\n")
						continue
					}
					filePath := parts[1]
					if strings.HasPrefix(filePath, "#") {
						index := parseIndex(filePath)
						if index == -1 {
							logMessage("Invalid index.\n")
							continue
						}
						fileManager.Mutex.Lock()
						if index >= len(fileManager.Files) {
							logMessage("Index out of range.\n")
							fileManager.Mutex.Unlock()
							continue
						}
						filePath = fileManager.Files[index].Path
						fileManager.Mutex.Unlock()
					} else {
						fileManager.Mutex.Lock()
						if !fileManager.contains(filePath) {
							fileInfo, err := os.Stat(filePath)
							if err != nil {
								logMessage("Error accessing file: %v\n", err)
								fileManager.Mutex.Unlock()
								continue
							}
							fileManager.Files = append(fileManager.Files, FileEntry{
								Path:    filePath,
								Size:    fileInfo.Size(),
								Watched: false,
							})
							logMessage("Added file: %s\n", filePath)
						}
						fileManager.Mutex.Unlock()
					}
					if err := watcher.Add(filePath); err != nil {
						watcher, err = fsnotify.NewWatcher()
						logMessage("Creating new watcher...\n")
						if err != nil {
							logMessage("Error creating watcher: %v\n", err)
							continue
						}
					}
					logMessage("Now watching: %s\n", filePath)
					fileManager.Mutex.Lock()
					for i := range fileManager.Files {
						if fileManager.Files[i].Path == filePath {
							fileManager.Files[i].Watched = true
							break
						}
					}
					fileManager.Mutex.Unlock()

				case "/woff":
					if len(parts) < 2 {
						logMessage("Usage: /woff <file> or /woff #<number>\n")
						continue
					}
					filePath := parts[1]
					if strings.HasPrefix(filePath, "#") {
						index := parseIndex(filePath)
						if index == -1 {
							logMessage("Invalid index.\n")
							continue
						}
						fileManager.Mutex.Lock()
						if index >= len(fileManager.Files) {
							logMessage("Index out of range.\n")
							fileManager.Mutex.Unlock()
							continue
						}
						filePath = fileManager.Files[index].Path
						fileManager.Mutex.Unlock()
					}
					if err := watcher.Remove(filePath); err != nil {
						logMessage("Error unwatching file: %v\n", err)
					} else {
						logMessage("Stopped watching: %s\n", filePath)
						fileManager.Mutex.Lock()
						for i := range fileManager.Files {
							if fileManager.Files[i].Path == filePath {
								fileManager.Files[i].Watched = false
								break
							}
						}
						fileManager.Mutex.Unlock()
					}

				case "/add":
					if len(parts) < 2 {
						logMessage("Usage: /add <file>\n")
						continue
					}
					filePath := parts[1]
					fileInfo, err := os.Stat(filePath)
					if err != nil {
						logMessage("Error accessing file: %v\n", err)
						continue
					}
					fileManager.Mutex.Lock()
					fileManager.Files = append(fileManager.Files, FileEntry{
						Path:    filePath,
						Size:    fileInfo.Size(),
						Watched: false,
					})
					fileManager.Mutex.Unlock()
					logMessage("Added file: %s\n", filePath)

				case "/ls":
					fileManager.Mutex.Lock()
					logMessage("Index | Watched | Size | Path\n")
					for i, file := range fileManager.Files {
						watchedStatus := "NO"
						if file.Watched {
							watchedStatus = "YES"
						}
						logMessage("%5d | %7s | %4d | %s\n", i, watchedStatus, file.Size, file.Path)
					}
					fileManager.Mutex.Unlock()

				case "/cl":
					clearConsole()

				default:
					logMessage(`
Unknown command. 
Available commands:
	- /add                       Add a file to the alias list
	- /ls                        List files ready to be uploaded
	- /cl                        Clear the console
	- /up <file> or #<number>    Upload a file
	- /w <file> or #<number>     Watch a file
	- /woff <file> or #<number>  Cancel watch for a file
`)
				}
			}
		}
	}()

	var (
		lastEventTime time.Time
		debounceDelay = 500 * time.Millisecond
	)
	for {
		select {
		case <-quit:
			return
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				filePath := event.Name

				// Check if the file was received from the peer
				receivedFilesMutex.Lock()
				if receivedFiles[filePath] {
					receivedFilesMutex.Unlock()
					continue // Ignore changes to received files
				}
				receivedFilesMutex.Unlock()

				if time.Since(lastEventTime) < debounceDelay {
					continue
				}
				lastEventTime = time.Now()

				// Ensure the file is fully written before uploading
				time.Sleep(100 * time.Millisecond) // Small delay to allow file write to complete

				// Check file size before uploading
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					logMessage("Error getting file info: %v\n", err)
					continue
				}

				if fileInfo.Size() == 0 {
					logMessage("File is empty, skipping upload: %s\n", filePath)
					continue
				}

				if err := sendFileWithProgress(filePath); err != nil {
					logMessage("Error uploading file: %v\n", err)
				} else {
					logMessage("File uploaded automatically: %s\n", filePath)
				}
			}
		case err := <-watcher.Errors:
			logMessage("Watcher error: %v\n", err)
		}
	}
}

func sendFileWithProgress(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	totalSize := fileInfo.Size()
	sentBytes := int64(0)

	// Use smaller chunks to avoid base64 overhead
	buffer := make([]byte, 1024*1024) // 1MB chunks

	for sentBytes < totalSize {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read error: %v", err)
		}
		if n == 0 {
			break
		}

		chunk := buffer[:n]
		message := Message{
			Action:    "upload",
			Path:      filepath.Base(filePath),
			Content:   base64.StdEncoding.EncodeToString(chunk),
			TotalSize: totalSize,
		}

		if err := sendMessage(message); err != nil {
			return fmt.Errorf("send error at %d/%d bytes: %v", sentBytes, totalSize, err)
		}

		sentBytes += int64(n)
		logMessage("\rUploading: %d/%d bytes (%d%%)", sentBytes, totalSize, (sentBytes*100)/totalSize)
	}

	if sentBytes != totalSize {
		return fmt.Errorf("incomplete transfer: sent %d/%d bytes", sentBytes, totalSize)
	}

	logMessage("\nFile transfer completed: %s (%d bytes)\n", filepath.Base(filePath), totalSize)
	return nil
}

func sendMessage(message Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = CurrentConn.Write(append(data, '\n'))
	return err
}

func readMessage(reader *bufio.Reader) (Message, error) {
	data, err := reader.ReadString('\n')
	if err != nil {
		return Message{}, err
	}

	var message Message
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		return Message{}, err
	}

	return message, nil
}

func parseIndex(s string) int {
	var index int
	_, err := fmt.Sscanf(s, "#%d", &index)
	if err != nil {
		return -1
	}
	return index
}

// Helper function to check if a file is already in the list
func (fm *FileManager) contains(filePath string) bool {
	for _, file := range fm.Files {
		if file.Path == filePath {
			return true
		}
	}
	return false
}

// Clear the console
func clearConsole() {
	cmd := exec.Command("clear") // Use "cls" for Windows
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// Log messages with timestamps
func logMessage(format string, a ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] "+format, append([]interface{}{timestamp}, a...)...)
}

func main() {
	config := loadConfig()

	if _, err := os.Stat(config.Folder); os.IsNotExist(err) {
		os.Mkdir(config.Folder, 0755)
	}

	if config.Mode == "host" {
		startHost(config)
	} else {
		connectToHost(config)
	}
}
