package command

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func files(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("files", flag.ContinueOnError)
	flags.SetOutput(out)
	dir := flags.String("dir", "", "Directory of game downloads (required)")
	listen := flags.String("listen", "127.0.0.1:8080", "HTTP listen address")
	tokenFile := flags.String("token-file", ".insta-game/token", "Token file for authenticated downloads")
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	if *dir == "" {
		return fmt.Errorf("--dir is required")
	}
	token, err := tokenAt(*tokenFile, false)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(*dir)
	if err != nil {
		return err
	}
	defer root.Close()
	// os.Root prevents symbolic links from escaping the selected directory.
	handler := http.FileServerFS(root.FS())
	fmt.Fprintf(out, "Read-only files on http://%s\n", *listen)
	return httpServe(ctx, *listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		handler.ServeHTTP(w, r)
	}))
}
