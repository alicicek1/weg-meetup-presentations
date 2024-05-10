package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/tools/godoc/static"
	"golang.org/x/tools/playground/socket"
	"golang.org/x/tools/present"

	// This will register handlers at /compile and /share that will proxy to the
	// respective endpoints at play.golang.org. This allows the frontend to call
	// these endpoints without needing cross-origin request sharing (CORS).
	// Note that this is imported regardless of whether the endpoints are used or
	// not (in the case of a local socket connection, they are not called).
	_ "golang.org/x/tools/playground"
)

var scripts = []string{"prism.js", "jquery-ui.js"}

func playScript(root, transport string) {
	modTime := time.Now()

	var buf bytes.Buffer

	for _, p := range scripts {
		if s, ok := static.Files[p]; ok {
			buf.WriteString(s)
			continue
		}

		b, err := ioutil.ReadFile(filepath.Join(root, "static", p))
		if err != nil {
			panic(err)
		}

		buf.Write(b)
	}

	fprintf, err := fmt.Fprintf(&buf, "\ninitPlayground(new %v());\n", transport)
	if err != nil {
		fmt.Println(fprintf)
		return
	}

	b := buf.Bytes()
	http.HandleFunc("/play.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/javascript")
		http.ServeContent(w, r, "", modTime, bytes.NewReader(b))
	})
}

func initPlayground(basepath string, origin *url.URL) {
	if !present.PlayEnabled {
		return
	}

	if *usePlayground {
		playScript(basepath, "HTTPTransport")
		return
	}

	if *nativeClient {
		// When specifying nativeClient, non-Go code cannot be executed
		// because the NaCl setup doesn't support doing so.
		socket.RunScripts = false
		socket.Environ = func() []string {
			if runtime.GOARCH == "amd64" {
				return environ("GOOS=nacl", "GOARCH=amd64p32")
			}

			return environ("GOOS=nacl")
		}
	}

	playScript(basepath, "SocketTransport")
	http.Handle("/socket", socket.NewHandler(origin))
}

func playable(c present.Code) bool {
	play := present.PlayEnabled && c.Play

	// Restrict playable files to only Go source files when using play.golang.org,
	// since there is no method to execute shell scripts there.
	if *usePlayground {
		return play && c.Ext == ".go"
	}

	return play
}

func environ(vars ...string) []string {
	env := os.Environ()

	for _, r := range vars {
		k := strings.SplitAfter(r, "=")[0]
		for i, v := range env {
			if strings.HasPrefix(v, k) {
				env[i] = r
				goto NEXT
			}
		}
		env = append(env, r)
	NEXT:
	}

	return env
}
