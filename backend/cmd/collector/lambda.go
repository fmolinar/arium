package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/news"
)

// lambdaRuntime reports whether the process was started by AWS Lambda, which
// sets AWS_LAMBDA_RUNTIME_API for custom runtimes (provided.al2023).
func lambdaRuntime() bool {
	return os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
}

// startLambda connects to MongoDB once per cold start and then serves one
// collector run per invocation (the scheduler sends one three times a day).
// Logs go to CloudWatch; Prometheus metrics aren't served, since nothing could
// scrape a function between invocations.
// Lambda's disk doesn't outlive the instance, so articles go straight to
// MongoDB, which also holds the dedup state. It returns only on a setup error.
//
// The connection string comes from mongoURI (env MONGO_URI) or, when that's
// empty, from the SSM SecureString named by MONGO_URI_PARAMETER, so it
// isn't visible in the function's configuration. With
// COLLECTOR_ARCHIVE_BUCKET set, each run's raw payloads and manifest are also
// copied to that S3 bucket (see archive).
func startLambda(ctx context.Context, c *collector.Collector, opts options, mongoURI, mongoDB string) error {
	// Credentials come from the function's role; loading the config makes no
	// network calls.
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}

	if mongoURI == "" {
		name := os.Getenv("MONGO_URI_PARAMETER")
		if name == "" {
			return errors.New("set MONGO_URI or MONGO_URI_PARAMETER")
		}

		if mongoURI, err = ssmParameter(ctx, cfg, name); err != nil {
			return err
		}
	}

	if bucket := os.Getenv("COLLECTOR_ARCHIVE_BUCKET"); bucket != "" {
		opts.archive = &archive{bucket: bucket, client: s3.NewFromConfig(cfg)}
	}

	client, err := mongo.Connect(mongooptions.Client().ApplyURI(mongoURI).SetServerSelectionTimeout(10 * time.Second))
	if err != nil {
		return fmt.Errorf("mongo: %w", err)
	}

	repo := news.NewRepository(client.Database(mongoDB))
	opts.news = news.NewService(repo)

	// Lambda may be the only writer (a fresh Atlas cluster), so it creates
	// the indexes the API and the title dedup rely on. That happens in the
	// first invocation rather than at cold start, which Lambda limits to 10s.
	indexed := false

	lambda.StartWithOptions(func(ctx context.Context) error {
		if !indexed {
			if err := repo.EnsureIndexes(ctx); err != nil {
				return fmt.Errorf("ensure indexes: %w", err)
			}
			indexed = true
		}

		return run(ctx, c, opts)
	}, lambda.WithContext(ctx))

	return nil
}

// ssmParameter returns the decrypted value of an SSM parameter.
func ssmParameter(ctx context.Context, cfg aws.Config, name string) (string, error) {
	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("read SSM parameter %s: %w", name, err)
	}

	return *out.Parameter.Value, nil
}
