package news

import (
	"context"
	"errors"
	"fmt"
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
		{Keys: bson.D{{Key: "title_key", Value: 1}}},
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

// InsertNew inserts the articles whose ID and title key aren't already in the
// collection or earlier in the batch, and leaves stored articles untouched, so
// fetched_at keeps the time an article was first seen. It is the MongoDB
// counterpart of the collector's file store dedup (collector.Store).
func (r *Repository) InsertNew(ctx context.Context, articles []Article) (IngestResult, error) {
	var result IngestResult
	if len(articles) == 0 {
		return result, nil
	}

	ids := make([]string, 0, len(articles))
	keys := []string{}
	for _, a := range articles {
		ids = append(ids, a.ID)
		if a.TitleKey != "" {
			keys = append(keys, a.TitleKey)
		}
	}

	seenIDs, err := r.distinct(ctx, "_id", ids)
	if err != nil {
		return result, err
	}
	seenKeys, err := r.distinct(ctx, "title_key", keys)
	if err != nil {
		return result, err
	}

	var fresh []Article
	for _, a := range articles {
		if seenIDs[a.ID] {
			result.DuplicatesByURL++
			continue
		}
		if a.TitleKey != "" && seenKeys[a.TitleKey] {
			result.DuplicatesByTitle++
			continue
		}

		fresh = append(fresh, a)
		seenIDs[a.ID] = true
		if a.TitleKey != "" {
			seenKeys[a.TitleKey] = true
		}
	}

	if len(fresh) == 0 {
		return result, nil
	}

	// An article inserted since the lookup above (a concurrent run) fails
	// with a duplicate key error; count it as a duplicate, not a failure.
	res, err := r.collection.InsertMany(ctx, fresh, options.InsertMany().SetOrdered(false))
	if res != nil {
		result.Inserted = int64(len(res.InsertedIDs))
	}

	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) && bwe.WriteConcernError == nil && onlyDuplicateKeys(bwe.WriteErrors) {
		result.DuplicatesByURL += int64(len(bwe.WriteErrors))
		return result, nil
	}

	return result, err
}

// distinct returns which of values are stored in field.
func (r *Repository) distinct(ctx context.Context, field string, values []string) (map[string]bool, error) {
	found := map[string]bool{}
	if len(values) == 0 {
		return found, nil
	}

	res := r.collection.Distinct(ctx, field, bson.M{field: bson.M{"$in": values}})
	var stored []string
	if err := res.Decode(&stored); err != nil {
		return nil, fmt.Errorf("look up existing %s: %w", field, err)
	}
	for _, v := range stored {
		found[v] = true
	}

	return found, nil
}

func onlyDuplicateKeys(errs []mongo.BulkWriteError) bool {
	for _, e := range errs {
		if e.Code != 11000 {
			return false
		}
	}

	return len(errs) > 0
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
