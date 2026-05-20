package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
	"github.com/wneessen/go-mail"
)

const (
	QueueName = "clipping_queue"
)

type Clipper struct {
	HttpClient      *http.Client
	GotenbergClient *gotenberg.Client
	PandocURL       string
	S3Client        *s3.Client
	S3Bucket        string
	MailClient      *mail.Client
	SMTPFrom        string
}

type ClippingPayload struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Format string `json:"format"`
	Email  string `json:"email"`
}

func (c *Clipper) HandleClipping(ctx context.Context, payload *ClippingPayload) error {
	// fail fast if email requested but SMTP not configured
	if payload.Email != "" && c.MailClient == nil {
		return errors.New("SMTP is not configured")
	}

	// fetch & clean
	cleanHTML, err := c.fetchAndClean(ctx, payload.URL)
	if err != nil {
		return err
	}

	// convert
	var fileBytes []byte

	switch payload.Format {
	case "pdf":
		reader, err := c.htmlToPDF(ctx, cleanHTML)
		if err != nil {
			return err
		}
		defer reader.Close()

		fileBytes, err = io.ReadAll(reader)
		if err != nil {
			return err
		}
	case "epub":
		reader, err := c.htmlToEPUB(ctx, cleanHTML)
		if err != nil {
			return err
		}
		defer reader.Close()

		fileBytes, err = io.ReadAll(reader)
		if err != nil {
			return err
		}

	case "html":
		fileBytes = cleanHTML
	default:
		return errors.New("unsupported format: " + payload.Format)
	}

	// upload
	_, err = c.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.S3Bucket),
		Key:         aws.String(payload.ID + "." + payload.Format),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentTypeFor(payload.Format)),
	})
	if err != nil {
		return err
	}

	// email
	if payload.Email != "" {
		filename := fmt.Sprintf("%s.%s", payload.ID, payload.Format)
		err = c.sendEmail(payload.Email, filename, fileBytes)
		if err != nil {
			return err
		}
	}

	return nil
}

func contentTypeFor(format string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "html":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

func (c *Clipper) fetchAndClean(ctx context.Context, targetURL string) ([]byte, error) {
	// fetch
	fetchReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	fetchReq.Header.Set("User-Agent", "readfriendly/1.0")

	fetchRes, err := c.HttpClient.Do(fetchReq)
	if err != nil {
		return nil, err
	}
	defer fetchRes.Body.Close()

	// clean
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	article, err := readability.FromReader(fetchRes.Body, parsedURL)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		return nil, err
	}

	title := article.Title()
	wrappedHTML := fmt.Sprintf(
		"<html><head><title>%s</title></head><body>%s</body></html>",
		title, buf.String(),
	)
	cleanHTML := []byte(wrappedHTML)

	return cleanHTML, nil
}

func (c *Clipper) htmlToPDF(ctx context.Context, htmlContent []byte) (io.ReadCloser, error) {
	doc, err := document.FromBytes("index.html", htmlContent)
	if err != nil {
		return nil, err
	}

	res, err := c.GotenbergClient.Send(ctx, gotenberg.NewHTMLRequest(doc))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("gotenberg failed with status code: " + res.Status)
	}

	return res.Body, nil
}

func (c *Clipper) htmlToEPUB(ctx context.Context, htmlContent []byte) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.PandocURL+"/api/convert/from/html/to/epub", bytes.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("Content-Disposition", `attachment; filename="index.html"`)

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("pandoc failed with status code: " + res.Status)
	}

	return res.Body, nil
}

func (c *Clipper) sendEmail(deliverTo string, filename string, fileBytes []byte) error {
	message := mail.NewMsg()
	err := message.From(c.SMTPFrom)
	if err != nil {
		return err
	}
	err = message.To(deliverTo)
	if err != nil {
		return err
	}
	message.Subject("Your clipping is ready")
	err = message.AttachReader(filename, bytes.NewReader(fileBytes))
	if err != nil {
		return err
	}

	return c.MailClient.DialAndSend(message)
}
