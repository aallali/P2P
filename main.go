package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/fsnotify/fsnotify"
)

const (
	ConfigFile    = "config.json"
	PingInterval  = 10 * time.Second // Increased ping interval
	PingTimeout   = 60 * time.Second // Increased ping timeout for large files
	ChunkSize     = 1024 * 1024      // 1MB chunks
	MaxRetries    = 5                // Increased max retries
	MaxConcurrent = 5                // Increased max concurrent chunks
	RateLimit     = 10 * 1024 * 1024 // Increased rate limit to 10MB per second
)

// Config structure
type Config struct {
	Mode   string `json:"mode"` // "host" or "peer"
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Folder string `json:"folder"`
}

// Message structure
type Message struct {
	Action    string `json:"action"`
	Path      string `json:"path"`
	Content   string `json:"content,omitempty"`
	ChunkNum  int    `json:"chunk_num,omitempty"`  // Current chunk number
	TotalSize int64  `json:"total_size,omitempty"` // Total file size
	Checksum  string `json:"checksum,omitempty"`
	Retry     int    `json:"retry,omitempty"`
	Busy      bool   `json:"busy,omitempty"` // Indicates if the peer is busy
}

func loadConfig() Config {
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		defaultConfig := Config{
			Mode:   "host",
			IP:     "0.0.0.0",
			Port:   12345,
			Folder: "./shared",
		}
		configData, _ := json.MarshalIndent(defaultConfig, "", "  ")
		ioutil.WriteFile(ConfigFile, configData, 0644)
		fmt.Printf("Config file created. Edit '%s' and rerun.\n", ConfigFile)
		os.Exit(0)
	}

	data, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		panic(err)
	}

	return config
}

func startHost(config Config) {
	var currentConn net.Conn       // Track the most recent connection
	lastPing := make(chan bool, 1) // Channel to track pong responses

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Printf("Hosting on %s:%d. Waiting for connection...\n", config.IP, config.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Replace the old connection with the new one
		if currentConn != nil {
			fmt.Println("Replacing existing connection with a new one.")
			currentConn.Close()
		}
		currentConn = conn

		fmt.Println("Peer connected.")
		notifyConnection(currentConn, "Peer connected")
		go handleConnection(config, currentConn, lastPing) // Handle connection in a goroutine
	}
}

func connectToHost(config Config) {
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
		if err != nil {
			fmt.Println("Host not available. Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		fmt.Println("Connected to host.")
		notifyConnection(conn, "Connected to host")
		lastPing := make(chan bool, 1) // Channel to track pong responses
		go handleConnection(config, conn, lastPing)
		monitorPing(conn, lastPing)
		return
	}
}

func notifyConnection(conn net.Conn, message string) {
	notif := Message{
		Action:  "notification",
		Content: message,
	}
	sendMessage(conn, notif)
	fmt.Println(message)
}

func monitorPing(conn net.Conn, lastPing chan bool) {
	go func() {
		for {
			select {
			case <-time.After(PingTimeout):
				fmt.Println("Connection lost: no response to ping.")
				conn.Close()
				return
			case <-lastPing:
				// Pong received, continue
			}
		}
	}()

	for {
		time.Sleep(PingInterval)
		ping := Message{Action: "ping"}
		sendMessage(conn, ping)
	}
}

func handleConnection(config Config, conn net.Conn, lastPing chan bool) {
	defer conn.Close()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	syncedFiles := make(map[string]time.Time) // Tracks recently synced files
	const syncIgnoreDuration = 2 * time.Second

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					if strings.HasSuffix(event.Name, "~") || strings.HasSuffix(event.Name, "tmp") {
						continue // Ignore temporary save files
					}
					relativePath, _ := filepath.Rel(config.Folder, event.Name)

					// Check if the file was recently synced
					if lastSync, found := syncedFiles[relativePath]; found && time.Since(lastSync) < syncIgnoreDuration {
						continue
					}

					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						continue
					}

					if fileInfo.Size() >= ChunkSize {
						go sendFile(conn, event.Name, relativePath)
					} else {
						content, _ := ioutil.ReadFile(event.Name)
						message := Message{
							Action:  "sync",
							Path:    relativePath,
							Content: string(content),
						}
						sendMessage(conn, message)
					}
				} else if event.Op&fsnotify.Remove != 0 {
					relativePath, _ := filepath.Rel(config.Folder, event.Name)
					message := Message{
						Action: "delete",
						Path:   relativePath,
					}
					sendMessage(conn, message)
				}
			case err := <-watcher.Errors:
				fmt.Println("Watcher error:", err)
			case <-done:
				return // Exit goroutine when done
			}
		}
	}()

	watcher.Add(config.Folder)

	for {
		message, err := readMessage(conn)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Peer disconnected.")
			} else {
				fmt.Println("Error reading message:", err)
			}
			break
		}

		if message.Action == "notification" {
			fmt.Println("Notification from peer:", message.Content)
			continue
		} else if message.Action == "ping" {
			pong := Message{Action: "pong"}
			sendMessage(conn, pong)
			continue
		} else if message.Action == "pong" {
			lastPing <- true
			continue
		}
		handleIncoming(config, message, syncedFiles, conn)
	}

	done <- true // Signal the goroutine to exit
}

// readMessage reads a message from the connection.
func readMessage(conn net.Conn) (Message, error) {
	// Read the message length prefix (4 bytes)
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return Message{}, err
	}

	// Read the actual message
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return Message{}, err
	}

	// Unmarshal the message
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}

	return message, nil
}

// sendMessage sends a message over the connection.
func sendMessage(conn net.Conn, message Message) error {
	// Marshal the message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the message length prefix (4 bytes)
	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return err
	}

	// Send the actual message
	if _, err := conn.Write(data); err != nil {
		return err
	}

	return nil
}

type FileReceiver struct {
	chunks    map[int][]byte
	totalSize int64
	path      string
	mutex     sync.Mutex
	checksum  map[int]string
}

var activeReceivers = make(map[string]*FileReceiver)

func handleIncoming(config Config, message Message, syncedFiles map[string]time.Time, conn net.Conn) {
	filePath := filepath.Join(config.Folder, message.Path)

	switch message.Action {
	case "file_chunk":
		receiver, exists := activeReceivers[message.Path]
		if !exists {
			receiver = &FileReceiver{
				chunks:    make(map[int][]byte),
				totalSize: message.TotalSize,
				path:      filePath,
				checksum:  make(map[int]string),
			}
			activeReceivers[message.Path] = receiver
		}

		content, err := base64.StdEncoding.DecodeString(message.Content)
		if err != nil {
			fmt.Printf("\rError decoding chunk %d: %v\n", message.ChunkNum, err)
			return
		}

		// Verify checksum
		hash := md5.Sum(content)
		if hex.EncodeToString(hash[:]) != message.Checksum {
			// Request retry
			retry := Message{
				Action:   "retry_chunk",
				Path:     message.Path,
				ChunkNum: message.ChunkNum,
			}
			sendMessage(conn, retry)
			return
		}

		receiver.mutex.Lock()
		receiver.chunks[message.ChunkNum] = content
		receiver.checksum[message.ChunkNum] = message.Checksum
		receiver.mutex.Unlock()

		// Calculate and display progress
		receivedSize := calculateProgress(receiver)
		progress := float64(receivedSize) / float64(receiver.totalSize) * 100
		fmt.Printf("\rReceiving %s: %.1f%%", message.Path, progress)

		// Check if file is complete
		if isFileComplete(receiver) {
			fmt.Println("\nFile received completely! Verifying...")
			if err := assembleAndSaveFile(receiver); err == nil {
				delete(activeReceivers, message.Path)
				syncedFiles[message.Path] = time.Now()
			}
		}

	case "sync":
		// For small files, use existing logic
		if len(message.Content) < ChunkSize {
			syncedFiles[message.Path] = time.Now() // Add to recently synced files
			os.MkdirAll(filepath.Dir(filePath), 0755)
			ioutil.WriteFile(filePath, []byte(message.Content), 0644)
			fmt.Printf("Synced: %s\n", filePath)
		} else {
			// For large files, initiate chunked transfer
			go sendFile(conn, filePath, message.Path)
		}

	case "delete":
		os.Remove(filePath)
		fmt.Printf("Deleted: %s\n", filePath)
	}
}

func calculateProgress(receiver *FileReceiver) int64 {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	var size int64
	for _, chunk := range receiver.chunks {
		size += int64(len(chunk))
	}
	return size
}

func isFileComplete(receiver *FileReceiver) bool {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	expectedChunks := (receiver.totalSize + ChunkSize - 1) / ChunkSize
	return int64(len(receiver.chunks)) == expectedChunks
}

func assembleAndSaveFile(receiver *FileReceiver) error {
	os.MkdirAll(filepath.Dir(receiver.path), 0755)
	file, err := os.Create(receiver.path)
	if err != nil {
		return err
	}
	defer file.Close()

	totalChunks := (receiver.totalSize + ChunkSize - 1) / ChunkSize
	for i := int64(0); i < totalChunks; i++ {
		file.Write(receiver.chunks[int(i)])
	}
	return nil
}

func sendFile(conn net.Conn, filePath, relativePath string) error {
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
	chunks := (totalSize + ChunkSize - 1) / ChunkSize // Round up division
	fmt.Printf("Sending file: %s (%.2f MB)\n", relativePath, float64(totalSize)/1024/1024)

	// Initialize rate limiter
	limiter := rate.NewLimiter(rate.Limit(RateLimit), ChunkSize)

	// Use semaphore for concurrent chunk control
	sem := make(chan bool, MaxConcurrent)
	var wg sync.WaitGroup

	for chunk := int64(0); chunk < chunks; chunk++ {
		wg.Add(1)
		sem <- true // Acquire semaphore

		go func(chunkNum int64) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			for retry := 0; retry < MaxRetries; retry++ {
				err := sendChunk(conn, file, chunkNum, totalSize, relativePath, limiter)
				if err == nil {
					break
				}
				fmt.Printf("\rRetrying chunk %d (%d/5)...", chunkNum, retry+1)
				time.Sleep(time.Second)
			}
		}(chunk)
	}

	wg.Wait()
	fmt.Println("\nFile sent successfully!")
	return nil
}

func sendChunk(conn net.Conn, file *os.File, chunkNum, totalSize int64, relativePath string, limiter *rate.Limiter) error {
	buffer := make([]byte, ChunkSize)
	file.Seek(chunkNum*ChunkSize, 0)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}

	// Wait for rate limiter
	limiter.WaitN(context.Background(), n)

	// Calculate checksum
	hash := md5.Sum(buffer[:n])
	checksum := hex.EncodeToString(hash[:])

	message := Message{
		Action:    "file_chunk",
		Path:      relativePath,
		Content:   base64.StdEncoding.EncodeToString(buffer[:n]),
		ChunkNum:  int(chunkNum),
		TotalSize: totalSize,
		Checksum:  checksum,
	}

	return sendMessage(conn, message)
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
