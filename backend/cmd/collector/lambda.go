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
// isn't visible in the function's configuration.
func startLambda(ctx context.Context, c *collector.Collector, opts options, mongoURI, mongoDB string) error {
	if mongoURI == "" {
		name := os.Getenv("MONGO_URI_PARAMETER")
		if name == "" {
			return errors.New("set MONGO_URI or MONGO_URI_PARAMETER")
		}

		var err error
		if mongoURI, err = ssmParameter(ctx, name); err != nil {
			return err
		}
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
func ssmParameter(ctx context.Context, name string) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("aws config: %w", err)
	}

	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("read SSM parameter %s: %w", name, err)
	}

	return *out.Parameter.Value, nil
}
