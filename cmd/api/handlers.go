package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/readeck/go-readability/v2"
)

func (app *application) getHealth(w http.ResponseWriter, r *http.Request) {
	type healthResponse struct {
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
	}
	res := healthResponse{
		Status:    "up",
		Timestamp: time.Now().UTC(),
	}

	err := writeJSON(w, http.StatusOK, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) postClipping(w http.ResponseWriter, r *http.Request) {
	// get request body
	type postClippingRequest struct {
		URL string `json:"url"`
	}
	var req postClippingRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()

	// fetch html
	fetchReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, req.URL, nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	fetchReq.Header.Set("User-Agent", "readfriendly/1.0")

	fetchRes, err := app.httpClient.Do(fetchReq)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer fetchRes.Body.Close()

	// clean html
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		app.serverError(w, err)
		return
	}
	article, err := readability.FromReader(fetchRes.Body, parsedURL)
	if err != nil {
		app.serverError(w, err)
		return
	}

	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		app.serverError(w, err)
		return
	}
	cleanHTML := buf.Bytes()

	// return clipping
	w.Header().Set("Content-Type", "text/html")
	w.Write(cleanHTML)
}
