package main

import (
	"embed"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/soypat/rebed"
	"golang.org/x/tools/present"
)

var (
	httpAddr      = flag.String("http", "127.0.0.1:2028", "HTTP service address (e.g., '127.0.0.1:2028')")
	originHost    = flag.String("orighost", "", "host component of web origin URL (e.g., 'localhost')")
	basePath      = flag.String("base", ".", "base path for slide template and static resources. default is current directory")
	contentPath   = flag.String("content", ".", "base path for presentation content")
	usePlayground = flag.Bool("use_playground", false, "run code snippets using play.golang.org; if false, run them locally and deliver results by WebSocket transport")
	nativeClient  = flag.Bool("nacl", false, "use Native Client environment playground (prevents non-Go code execution) when using local WebSocket transport")
)

// Embedded directories
var (
	//go:embed static
	staticFS embed.FS
)

func main() {
	flag.BoolVar(&present.PlayEnabled, "play", true, "enable playground (permit execution of arbitrary user code)")
	flag.BoolVar(&present.NotesEnabled, "notes", false, "enable presenter notes (press 'N' from the browser to display them)")

	rebed.Patch(staticFS, "")

	err := initTemplates(*basePath)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	origin := &url.URL{Scheme: "http"}
	if *originHost != "" {
		if strings.HasPrefix(*originHost, "https://") {
			*originHost = strings.TrimPrefix(*originHost, "https://")
			origin.Scheme = "https"
		}
		*originHost = strings.TrimPrefix(*originHost, "http://")
		origin.Host = net.JoinHostPort(*originHost, port)
	} else if ln.Addr().(*net.TCPAddr).IP.IsUnspecified() {
		name, _ := os.Hostname()
		origin.Host = net.JoinHostPort(name, port)
	} else {
		reqHost, reqPort, err := net.SplitHostPort(*httpAddr)
		if err != nil {
			log.Fatal(err)
		}
		if reqPort == "0" {
			origin.Host = net.JoinHostPort(reqHost, port)
		} else {
			origin.Host = *httpAddr
		}
	}

	initPlayground(*basePath, origin)
	http.Handle("/static/", http.FileServer(http.Dir(*basePath)))

	if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() &&
		present.PlayEnabled && !*nativeClient && !*usePlayground {
		log.Print("app is not running on localhost, the playground may not work")
	}

	log.Printf("Open your web browser and visit %s", origin.String())
	log.Fatal(http.Serve(ln, nil))
}
