package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
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
		URL    string `json:"url"`
		Format string `json:"format"` // 'pdf', 'epub', 'html'
	}
	var req postClippingRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()

	// validate format
	if req.Format != "pdf" && req.Format != "epub" && req.Format != "html" {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
		return
	}

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

	title := article.Title()
	wrappedHTML := fmt.Sprintf(
		"<html><head><title>%s</title></head><body>%s</body></html>",
		title, buf.String(),
	)
	cleanHTML := []byte(wrappedHTML)

	switch req.Format {
	case "pdf":
		pdfReader, err := app.htmlToPDF(r.Context(), cleanHTML)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer pdfReader.Close()

		pdfBytes, err := io.ReadAll(pdfReader)
		if err != nil {
			app.serverError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdfBytes)
	case "epub":
		epubReader, err := app.htmlToEPUB(r.Context(), cleanHTML)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer epubReader.Close()

		epubBytes, err := io.ReadAll(epubReader)
		if err != nil {
			app.serverError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/epub+zip")
		w.Write(epubBytes)
	case "html":
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write(cleanHTML)
		if err != nil {
			app.logger.Error(err.Error())
		}
	}
}

func (app *application) htmlToPDF(ctx context.Context, htmlContent []byte) (io.ReadCloser, error) {
	doc, err := document.FromBytes("index.html", htmlContent)
	if err != nil {
		return nil, err
	}

	res, err := app.gotenbergClient.Send(ctx, gotenberg.NewHTMLRequest(doc))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("gotenberg failed with status code: " + res.Status)
	}

	return res.Body, nil
}

func (app *application) htmlToEPUB(ctx context.Context, htmlContent []byte) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, app.pandocURL+"/api/convert/from/html/to/epub", bytes.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("Content-Disposition", `attachment; filename="index.html"`)

	res, err := app.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("pandoc failed with status code: " + res.Status)
	}

	return res.Body, nil
}
