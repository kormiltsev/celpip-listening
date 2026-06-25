package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.String("port", "8080", "Port to listen on")
	dir := flag.String("dir", "", "Directory to serve (default: executable directory)")
	flag.Parse()

	var absDir string
	var err error
	if *dir != "" {
		absDir, err = filepath.Abs(*dir)
		if err != nil {
			log.Fatalf("failed to resolve directory: %v", err)
		}
	} else {
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("failed to determine executable path: %v", err)
		}
		absDir = filepath.Dir(exePath)
		// Resolve symlinks
		if realPath, err := filepath.EvalSymlinks(absDir); err == nil {
			absDir = realPath
		}
	}

	info, err := os.Stat(absDir)
	if err != nil {
		log.Fatalf("directory does not exist: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("path is not a directory: %s", absDir)
	}

	fileServer := http.FileServer(http.Dir(absDir))

	mux := http.NewServeMux()
	mux.Handle("/", loggingMiddleware(fileServer))

	addr := ":" + *port

	fmt.Printf("Serving %s\n", absDir)
	fmt.Printf("Open http://localhost:%s\n", *port)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
