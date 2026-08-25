package webui

import (
	"io/fs"
	"net/http"
)

func Handler() http.Handler {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			page, readErr := fs.ReadFile(sub, "index.html")
			if readErr != nil {
				http.Error(w, "页面资源不可用", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
			return
		}
		files.ServeHTTP(w, r)
	})
}
