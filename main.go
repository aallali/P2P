package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const ConfigFile = "config.json"
const PingInterval = 5 * time.Second // Time between pings
const PingTimeout = 10 * time.Second // Time to wait for a pong before declaring disconnect

// Config structure
type Config struct {
	Mode   string `json:"mode"` // "host" or "peer"
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Folder string `json:"folder"`
}

// Message structure
type Message struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
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
	data, _ := json.Marshal(notif)
	conn.Write(append(data, '\n'))
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

					content, _ := ioutil.ReadFile(event.Name)
					message := Message{
						Action:  "sync",
						Path:    relativePath,
						Content: string(content),
					}
					sendMessage(conn, message)
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
			}
		}
	}()

	watcher.Add(config.Folder)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var message Message
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			fmt.Println("Error decoding message:", err)
			continue
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
		handleIncoming(config, message, syncedFiles)
	}

	fmt.Println("Peer disconnected.")
	done <- true
}

func sendMessage(conn net.Conn, message Message) {
	data, _ := json.Marshal(message)
	conn.Write(append(data, '\n'))
}

func handleIncoming(config Config, message Message, syncedFiles map[string]time.Time) {
	filePath := filepath.Join(config.Folder, message.Path)

	switch message.Action {
	case "sync":
		syncedFiles[message.Path] = time.Now() // Add to recently synced files
		os.MkdirAll(filepath.Dir(filePath), 0755)
		ioutil.WriteFile(filePath, []byte(message.Content), 0644)
		fmt.Printf("Synced: %s\n", filePath)
	case "delete":
		os.Remove(filePath)
		fmt.Printf("Deleted: %s\n", filePath)
	}
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
