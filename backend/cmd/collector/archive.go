package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/news"
)

// objectPutter is the part of the S3 client the archive uses.
type objectPutter interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// archive copies each Lambda run's raw payloads and manifest to S3, under the
// same keys the file store uses on disk (collector.RawKey, ManifestKey). The
// articles themselves live in MongoDB; the raw payloads let them be rebuilt
// after a parser or tagger change, and the manifests record what each run did.
//
// Unlike the file store it keeps every payload, changed or not: there is no
// state between invocations to compare against, and the bucket's lifecycle
// rule expires old objects.
type archive struct {
	bucket string
	client objectPutter
}

// save writes the run's raw payloads, then its manifest. It carries on past a
// failed write and returns every error joined, so a caller can log them and
// keep the run's outcome, which depends on MongoDB only.
func (a *archive) save(ctx context.Context, result collector.Result, ingested news.IngestResult, finished time.Time) error {
	var errs []error
	sources := slices.Clone(result.Sources)

	for i, raw := range result.Raw {
		if sources[i].Error != "" || len(raw.Data) == 0 {
			continue
		}

		key, err := collector.RawKey(result.Run, sources[i].Name, raw.Ext)
		if err == nil {
			err = a.put(ctx, key, raw.Data, contentType(raw.Ext))
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("raw payload for %s: %w", sources[i].Name, err))
			continue
		}
		sources[i].RawPath = key
	}

	m := collector.Manifest{
		RunID:             result.Run.ID,
		StartedAt:         result.Run.StartedAt,
		FinishedAt:        finished.UTC(),
		Sources:           sources,
		ArticlesWritten:   int(ingested.Inserted),
		DuplicatesByURL:   int(ingested.DuplicatesByURL),
		DuplicatesByTitle: int(ingested.DuplicatesByTitle),
	}

	key, err := collector.ManifestKey(m.RunID)
	if err == nil {
		var data []byte
		if data, err = json.MarshalIndent(m, "", "  "); err == nil {
			err = a.put(ctx, key, append(data, '\n'), "application/json")
		}
	}
	if err != nil {
		errs = append(errs, fmt.Errorf("manifest: %w", err))
	}

	return errors.Join(errs...)
}

func (a *archive) put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &a.bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	})

	return err
}

func contentType(ext string) string {
	switch ext {
	case "xml":
		return "application/xml"
	case "json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
