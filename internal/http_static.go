package internal

import (
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/go-chi/chi"
)

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		indexHTML, err := os.ReadFile("./static/index.html")
		if err != nil {
			w.Write([]byte(err.Error()))
		}
		w.Write(indexHTML)
	})
	r.Get("/static/script.js", func(w http.ResponseWriter, r *http.Request) {
		scriptJS, err := os.ReadFile("./static/script.js")
		if err != nil {
			log.Fatal(err)
		}
		scriptJSTemplate := template.Must(template.New("").Parse(string(scriptJS)))

		if err := scriptJSTemplate.Execute(w, "ws://"+r.Host+"/websocket"); err != nil {
			log.Fatal(err)
		}
	})
}
