package news

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{collection: db.Collection("news")}
}

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "published_at", Value: -1}, {Key: "_id", Value: -1}}},
		{Keys: bson.D{{Key: "tags", Value: 1}, {Key: "published_at", Value: -1}, {Key: "_id", Value: -1}}},
		{Keys: bson.D{{Key: "fetched_at", Value: 1}}},
	})

	return err
}

// Upsert inserts or replaces articles by ID, so importing the same article
// twice is harmless.
func (r *Repository) Upsert(ctx context.Context, articles []Article) (upserted, updated int64, err error) {
	if len(articles) == 0 {
		return 0, 0, nil
	}

	models := make([]mongo.WriteModel, 0, len(articles))
	for _, a := range articles {
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": a.ID}).
			SetReplacement(a).
			SetUpsert(true))
	}

	result, err := r.collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, 0, err
	}

	return result.UpsertedCount, result.ModifiedCount, nil
}

// DeleteFetchedBefore removes articles fetched before cutoff.
func (r *Repository) DeleteFetchedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.collection.DeleteMany(ctx, bson.M{"fetched_at": bson.M{"$lt": cutoff}})
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// List returns up to q.Limit articles, newest first.
func (r *Repository) List(ctx context.Context, q ListQuery) ([]Article, error) {
	filter := bson.D{}
	if q.Tag != "" {
		filter = append(filter, bson.E{Key: "tags", Value: q.Tag})
	}
	if q.After != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.M{"published_at": bson.M{"$lt": q.After.PublishedAt}},
			bson.M{"published_at": q.After.PublishedAt, "_id": bson.M{"$lt": q.After.ID}},
		}})
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "published_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(q.Limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	articles := []Article{}
	if err := cursor.All(ctx, &articles); err != nil {
		return nil, err
	}

	return articles, nil
}
