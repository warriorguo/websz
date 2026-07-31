package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/warriorguo/websz/internal/server"
	"github.com/warriorguo/websz/internal/updater"
	"github.com/warriorguo/websz/web"
)

func main() {
	var (
		root     = flag.String("root", "", "Root directory (default: current working directory)")
		listen   = flag.String("listen", "0.0.0.0:18090", "Listen address")
		token    = flag.String("token", "", "Access token (default: none, auto-generated for non-localhost)")
		readonly = flag.Bool("readonly", false, "Read-only mode")
		help    = flag.Bool("help", false, "Show help")
		version     = flag.Bool("version", false, "Print version and exit")
		checkUpdate = flag.Bool("check-update", false, "Check latest version available")
		update      = flag.Bool("update", false, "Update to latest version")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	if *version {
		updater.PrintVersion()
		return
	}

	if *checkUpdate {
		if err := updater.CheckLatest(); err != nil {
			log.Fatalf("Check update failed: %v", err)
		}
		return
	}

	if *update {
		if err := updater.Update(); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
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
		generated, err := generateToken()
		if err != nil {
			// Fail closed: the alternative is exposing the filesystem to the
			// network with no credential at all.
			log.Fatalf("Failed to generate an access token: %v", err)
		}
		accessToken = generated
		log.Printf("Generated access token: %s", accessToken)
		log.Printf("Use this token in the X-Websz-Token header or the ?t= query parameter")
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

// generateToken returns a URL-safe random token.
//
// This token is the only thing standing between the network and full read/write
// access to the served directory, so it uses crypto/rand and 16 bytes of
// entropy. The previous implementation drew 6 characters from math/rand seeded
// with the current time, which was both brute-forcible (62^6) and predictable
// from an approximate process start time.
func generateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// RawURLEncoding keeps the token safe in query strings and cookies without
	// padding: 16 bytes becomes 22 characters of [A-Za-z0-9-_].
	return base64.RawURLEncoding.EncodeToString(buf), nil
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