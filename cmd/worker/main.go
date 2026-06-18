package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/wneessen/go-mail"
	"github.com/yildiz-fatih/readfriendly/internal/models"
	"github.com/yildiz-fatih/readfriendly/internal/services"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	// get environment variables
	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		logger.Error("GOTENBERG_URL is not set")
		os.Exit(1)
	}

	pandocURL := os.Getenv("PANDOC_URL")
	if pandocURL == "" {
		logger.Error("PANDOC_URL is not set")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	s3BucketName := os.Getenv("S3_BUCKET_NAME")
	if s3BucketName == "" {
		logger.Error("S3_BUCKET_NAME is not set")
		os.Exit(1)
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		logger.Error("RABBITMQ_URL is not set")
		os.Exit(1)
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := mail.DefaultPortTLS // sensible default
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr != "" {
		var err error
		smtpPort, err = strconv.Atoi(smtpPortStr)
		if err != nil {
			logger.Error("SMTP_PORT is not a valid integer")
			os.Exit(1)
		}
	}
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	// init

	// http client
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// gotenberg client
	gotenbergClient, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// postgres
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("connected to database")

	clippingModel := &models.ClippingModel{DB: db}

	// s3
	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.DisableS3ExpressSessionAuth = aws.Bool(false)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	// email
	var mailClient *mail.Client
	if smtpHost != "" {
		mailClient, err = mail.NewClient(
			smtpHost,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithPort(smtpPort),
			mail.WithUsername(smtpUser),
			mail.WithPassword(smtpPass),
		)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}

	clipper := &services.Clipper{
		HttpClient:      httpClient,
		GotenbergClient: gotenbergClient,
		PandocURL:       pandocURL,
		S3Client:        s3Client,
		S3Bucket:        s3BucketName,
		MailClient:      mailClient,
		SMTPFrom:        smtpFrom,
	}

	// rabbitmq
	rabbitConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer rabbitConn.Close()

	rabbitCh, err := rabbitConn.Channel()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer rabbitCh.Close()

	q, err := rabbitCh.QueueDeclare(
		services.QueueName, // name
		true,               // durability
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	err = rabbitCh.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	msgs, err := rabbitCh.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	logger.Info("starting worker")

	for d := range msgs {
		var payload services.ClippingPayload
		err := json.Unmarshal(d.Body, &payload)
		if err != nil {
			logger.Error(err.Error())
			continue
		}

		err = clipper.HandleClipping(context.Background(), &payload)
		if err != nil {
			logger.Error("failed to handle clipping", "error", err.Error())
			rabbitCh.Nack(d.DeliveryTag, false, false) // drop if fails (TODO)
			// update status in database
			err = clippingModel.Update(payload.ID, models.StatusFailed)
			if err != nil {
				logger.Error(err.Error())
			}
			continue
		}
		rabbitCh.Ack(d.DeliveryTag, false)
		// update status in database
		err = clippingModel.Update(payload.ID, models.StatusCompleted)
		if err != nil {
			logger.Error(err.Error())
		}
	}
}
