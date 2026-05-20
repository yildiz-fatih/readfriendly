package main

import "net/http"

func (app *application) newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", app.getHealth)
	mux.HandleFunc("POST /clippings", app.postClipping)

	return app.recoverPanic(mux)
}
