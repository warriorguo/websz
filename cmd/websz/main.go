package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/warriorguo/websz/internal/server"
	"github.com/warriorguo/websz/web"
)

func main() {
	var (
		root     = flag.String("root", "", "Root directory (default: current working directory)")
		listen   = flag.String("listen", "0.0.0.0:18090", "Listen address")
		token    = flag.String("token", "", "Access token (default: none, recommended for non-localhost)")
		readonly = flag.Bool("readonly", false, "Read-only mode")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	rootDir := *root
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current working directory: %v", err)
		}
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		log.Fatalf("Invalid root path: %v", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		log.Fatalf("Root directory does not exist: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Root path is not a directory: %s", absRoot)
	}

	accessToken := *token
	if accessToken == "" && !isLocalhost(*listen) {
		accessToken = generateToken()
		log.Printf("Generated access token: %s", accessToken)
		log.Printf("Use this token in X-Websz-Token header or ?token= query parameter")
	}

	config := &server.Config{
		Root:     absRoot,
		Token:    accessToken,
		ReadOnly: *readonly,
	}

	srv, err := server.NewServer(config, web.StaticFiles)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("websz File Manager")
	log.Printf("Root: %s", absRoot)
	log.Printf("Listen: %s", *listen)
	log.Printf("Read-only: %v", *readonly)
	if accessToken != "" {
		log.Printf("Token required: %s", accessToken)
	}
	log.Printf("")
	
	printAccessURLs(*listen, accessToken)
	
	log.Printf("Starting server...")
	if err := http.ListenAndServe(*listen, srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func printHelp() {
	fmt.Println("websz - Local directory file transfer and management web tool")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  websz [options]")
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  websz")
	fmt.Println("  websz -root /home/user/files")
	fmt.Println("  websz -listen 127.0.0.1:8080")
	fmt.Println("  websz -token mytoken123 -readonly")
}

func isLocalhost(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "localhost"
}

func generateToken() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, 6)
	for i := range bytes {
		bytes[i] = charset[rand.Intn(len(charset))]
	}
	return string(bytes)
}

func printAccessURLs(listen, token string) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		log.Printf("Access URL: http://%s", listen)
		return
	}

	if host == "0.0.0.0" {
		log.Printf("Access URLs:")
		log.Printf("  Local:    http://127.0.0.1:%s", port)
		
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						log.Printf("  Network:  http://%s:%s", ipnet.IP, port)
					}
				}
			}
		}
	} else {
		log.Printf("Access URL: http://%s", listen)
	}
	log.Printf("")
}