package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yildiz-fatih/readfriendly/internal/models"
	"github.com/yildiz-fatih/readfriendly/internal/services"
)

type postClippingRequest struct {
	URL    string                `json:"url"`
	Format models.ClippingFormat `json:"format"`
	Email  string                `json:"email"` // optional
}

type postClippingResponse struct {
	ID     string                `json:"id"`
	Status models.ClippingStatus `json:"status"`
}

type getClippingResponse struct {
	Status      models.ClippingStatus `json:"status,omitempty"`
	DownloadURL string                `json:"download_url,omitempty"`
}

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

// @Summary	Queues a webpage to be clipped and delivered to an email address in a specified format
// @Accept		json
// @Produce	json
// @Param		request	body		postClippingRequest	true	"Clipping request"
// @Success	202		{object}	postClippingResponse
// @Failure	400		{object}	map[string]string
// @Failure	500		{object}	map[string]string
// @Router		/clippings [post]
func (app *application) postClipping(w http.ResponseWriter, r *http.Request) {
	// get request body
	var req postClippingRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()

	// validate format
	if req.Format != models.FormatPDF && req.Format != models.FormatEPUB && req.Format != models.FormatHTML {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
		return
	}

	// make a unique id for the task
	id := uuid.NewString()

	// enqueue the task
	payload := services.ClippingPayload{
		ID:     id,
		URL:    req.URL,
		Format: string(req.Format),
		Email:  req.Email,
	}
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		app.serverError(w, err)
		return
	}

	rabbitCh, err := app.rabbitConnection.Channel()
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer rabbitCh.Close()

	err = rabbitCh.PublishWithContext(r.Context(),
		"",                 // exchange
		services.QueueName, // routing key
		false,              // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         payloadJson,
		})
	if err != nil {
		app.serverError(w, err)
		return
	}

	// write to database
	clipping, err := app.clippingModel.Insert(id, req.Format)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// return immediately
	res := postClippingResponse{
		ID:     id,
		Status: clipping.Status,
	}
	err = writeJSON(w, http.StatusAccepted, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

// @Summary	Checks the status of a clipping job (and provides a download URL when ready)
// @Produce	json
// @Param		id	path		string	true	"Clipping ID"
// @Success	200	{object}	getClippingResponse
// @Failure	404	{object}	map[string]string
// @Failure	500	{object}	map[string]string
// @Router		/clippings/{id} [get]
func (app *application) getClipping(w http.ResponseWriter, r *http.Request) {
	// get url path
	id := r.PathValue("id")

	// check task status
	clipping, err := app.clippingModel.Get(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			app.clientError(w, http.StatusNotFound, "clipping not found")
			return
		}
		app.serverError(w, err)
		return
	}

	switch clipping.Status {
	case models.StatusCompleted:
		presignedReq, err := app.s3PresignClient.PresignGetObject(r.Context(), &s3.GetObjectInput{
			Bucket: aws.String(app.s3Bucket),
			Key:    aws.String(id + "." + string(clipping.Format)),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(1 * time.Hour)
		})
		if err != nil {
			app.serverError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, nil, getClippingResponse{DownloadURL: presignedReq.URL})
		return
	case models.StatusPending:
		writeJSON(w, http.StatusOK, nil, getClippingResponse{Status: models.StatusPending})
		return
	case models.StatusFailed:
		writeJSON(w, http.StatusOK, nil, getClippingResponse{Status: models.StatusFailed})
		return
	}
}
