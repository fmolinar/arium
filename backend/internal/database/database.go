package database

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
	"go.opentelemetry.io/otel/trace"
)

// Connect connects to MongoDB and pings it. Every command gets a client span
// from tracer, a child of the request span in its context, and is timed in
// the db.client.operation.duration metric on the global meter provider.
// Command bodies aren't recorded, since they hold user data.
func Connect(ctx context.Context, uri string, tracer trace.TracerProvider) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetMonitor(otelmongo.NewMonitor(otelmongo.WithTracerProvider(tracer), otelmongo.WithCommandAttributeDisabled(true))))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}
