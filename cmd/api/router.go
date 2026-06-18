package main

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/yildiz-fatih/readfriendly/docs"
)

func (app *application) newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/docs/", httpSwagger.WrapHandler)
	mux.HandleFunc("GET /health", app.getHealth)
	mux.HandleFunc("POST /clippings", app.postClipping)
	mux.HandleFunc("GET /clippings/{id}", app.getClipping)

	return app.recoverPanic(mux)
}
