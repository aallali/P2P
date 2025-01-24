package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
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
	Mode   string `json:"mode"` // "host" or "peer"
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Folder string `json:"folder"`
}

// Message structure
type Message struct {
	Action  string `json:"action"`  // "upload", "notification"
	Path    string `json:"path"`    // File path
	Content string `json:"content"` // File content (base64 encoded)
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

func startHost(config Config) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Printf("Hosting on %s:%d. Waiting for connection...\n", config.IP, config.Port)

	var (
		currentConn net.Conn
		connMutex   sync.Mutex
	)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Close the previous connection if it exists
		connMutex.Lock()
		if currentConn != nil {
			currentConn.Close()
		}
		currentConn = conn
		connMutex.Unlock()

		fmt.Println("Peer connected.")

		// Handle the connection in a new goroutine
		go handleConnection(config, conn, &connMutex, &currentConn)
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
		defer conn.Close()
		fmt.Println("Connected to host.")

		// Handle the connection
		handleConnection(config, conn, nil, nil)
	}
}

func handleConnection(config Config, conn net.Conn, connMutex *sync.Mutex, currentConn *net.Conn) {
	defer func() {
		fmt.Println("Peer disconnected.")
		conn.Close()

		// Clear the current connection if this is the host
		if connMutex != nil && currentConn != nil {
			connMutex.Lock()
			if *currentConn == conn {
				*currentConn = nil
			}
			connMutex.Unlock()
		}
	}()

	// Notify the other peer that we're connected
	sendMessage(conn, Message{Action: "notification", Content: "Connected!"})

	// Track files received from the peer to avoid recursive uploads
	receivedFiles := make(map[string]bool)
	var receivedFilesMutex sync.Mutex

	// Start a file watcher for the /watch command
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating watcher:", err)
		return
	}
	defer watcher.Close()

	// Handle incoming messages
	go func() {
		reader := bufio.NewReader(conn)
		for {
			message, err := readMessage(reader)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Peer disconnected.")
				} else {
					fmt.Println("Error reading message:", err)
				}
				return
			}

			switch message.Action {
			case "upload":
				// Save the uploaded file
				filePath := filepath.Join(config.Folder, message.Path)
				os.MkdirAll(filepath.Dir(filePath), 0755)
				if err := os.WriteFile(filePath, []byte(message.Content), 0644); err != nil {
					fmt.Println("Error saving file:", err)
				} else {
					fmt.Printf("File saved: %s\n", filePath)

					// Mark the file as received to avoid recursive uploads
					receivedFilesMutex.Lock()
					receivedFiles[filePath] = true
					receivedFilesMutex.Unlock()

					// Clear the received flag after a short delay
					go func() {
						time.Sleep(2 * time.Second) // Adjust delay as needed
						receivedFilesMutex.Lock()
						delete(receivedFiles, filePath)
						receivedFilesMutex.Unlock()
					}()
				}
			case "notification":
				// Print notifications
				fmt.Println("Notification from peer:", message.Content)
			}
		}
	}()

	// Read commands from the user
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			command := scanner.Text()
			if strings.HasPrefix(command, "/upload ") {
				// Upload a file
				filePath := strings.TrimPrefix(command, "/upload ")
				if err := sendFile(conn, filePath, connMutex, currentConn); err != nil {
					fmt.Println("Error uploading file:", err)
				} else {
					fmt.Println("File uploaded successfully!")
				}
			} else if strings.HasPrefix(command, "/watch ") {
				// Watch a file for changes
				filePath := strings.TrimPrefix(command, "/watch ")
				if err := watcher.Add(filePath); err != nil {
					fmt.Println("Error watching file:", err)
				} else {
					fmt.Printf("Now watching: %s\n", filePath)
				}
			} else {
				fmt.Println("Unknown command. Use '/upload <file>' or '/watch <file>'.")
			}
		}
	}()

	// Handle file watcher events
	for {
		select {
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

				// Upload the file to the peer
				if err := sendFile(conn, filePath, connMutex, currentConn); err != nil {
					fmt.Println("Error uploading file:", err)
				} else {
					fmt.Printf("File uploaded automatically: %s\n", filePath)
				}
			}
		case err := <-watcher.Errors:
			fmt.Println("Watcher error:", err)
		}
	}
}

func sendFile(conn net.Conn, filePath string, connMutex *sync.Mutex, currentConn *net.Conn) error {
	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Send the file as a message
	message := Message{
		Action:  "upload",
		Path:    filepath.Base(filePath),
		Content: string(content),
	}

	// If this is the host, use the current connection
	if connMutex != nil && currentConn != nil {
		connMutex.Lock()
		if *currentConn == nil {
			connMutex.Unlock()
			return fmt.Errorf("no active connection")
		}
		conn = *currentConn
		connMutex.Unlock()
	}

	return sendMessage(conn, message)
}

func sendMessage(conn net.Conn, message Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = conn.Write(append(data, '\n'))
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
