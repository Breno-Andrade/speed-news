// Command fixture-server serves fake versions of every external source for the E2E suite.
// It writes a news sources YAML pointing at itself, then prints "FIXTURE_SERVER_READY port=<port>".
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"speed-news/apps/api/internal/fixtures"
)

func main() {
	listenPort := flag.Int("port", 0, "listen port; 0 picks a free one")
	sourcesFilePath := flag.String("write-sources-file", "", "write a news sources YAML pointing at this server")
	flag.Parse()

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *listenPort))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture-server:", err)
		os.Exit(1)
	}
	listeningPort := listener.Addr().(*net.TCPAddr).Port

	if *sourcesFilePath != "" {
		sourcesYAML := fixtures.SourcesYAML(fmt.Sprintf("http://127.0.0.1:%d", listeningPort))
		if err := os.WriteFile(*sourcesFilePath, []byte(sourcesYAML), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "fixture-server:", err)
			os.Exit(1)
		}
	}

	fmt.Printf("FIXTURE_SERVER_READY port=%d\n", listeningPort)
	httpServer := &http.Server{Handler: fixtures.NewServer(time.Now).Handler(), ReadHeaderTimeout: 5 * time.Second}
	if err := httpServer.Serve(listener); err != nil {
		fmt.Fprintln(os.Stderr, "fixture-server:", err)
		os.Exit(1)
	}
}
