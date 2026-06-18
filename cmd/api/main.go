package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yildiz-fatih/readfriendly/internal/models"
	"github.com/yildiz-fatih/readfriendly/internal/services"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 20 * time.Second
	idleTimeout       = 120 * time.Second
)

type application struct {
	logger           *slog.Logger
	httpClient       *http.Client
	clippingModel    *models.ClippingModel
	s3PresignClient  *s3.PresignClient
	s3Bucket         string
	rabbitConnection *amqp.Connection
}

// @title			ReadFriendly API
// @version		1.0
// @description	Makes webpages friendly to read
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	// get environment variables
	port := os.Getenv("PORT")
	if port == "" {
		logger.Error("PORT is not set")
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

	// init

	// http client
	httpClient := &http.Client{Timeout: 30 * time.Second}

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

	s3PresignClient := s3.NewPresignClient(s3Client)

	// rabbitmq
	rabbitConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer rabbitConn.Close()

	ch, err := rabbitConn.Channel()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
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

	app := &application{
		logger:           logger,
		httpClient:       httpClient,
		clippingModel:    &models.ClippingModel{DB: db},
		s3PresignClient:  s3PresignClient,
		s3Bucket:         s3BucketName,
		rabbitConnection: rabbitConn,
	}

	// server
	err = app.serve(":" + port)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
