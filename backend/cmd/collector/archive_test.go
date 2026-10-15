package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/news"
)

// fakeS3 records PutObject calls and fails those whose key is in fail.
type fakeS3 struct {
	objects map[string][]byte
	types   map[string]string
	fail    map[string]bool
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.fail[*in.Key] {
		return nil, errors.New("access denied")
	}

	data, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.objects[*in.Key] = data
	f.types[*in.Key] = *in.ContentType

	return &s3.PutObjectOutput{}, nil
}

func TestArchiveSave(t *testing.T) {
	started := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	run := collector.Run{ID: "20261014T080000Z-abc123", StartedAt: started}

	result := collector.Result{
		Run: run,
		Sources: []collector.SourceResult{
			{Name: "cncf", Fetched: 3},
			{Name: "hackernews", Fetched: 1},
			{Name: "broken", Error: "timeout"},
			{Name: "denied", Fetched: 2},
		},
		Raw: []collector.Raw{
			{Data: []byte("<rss/>"), Ext: "xml"},
			{Data: []byte(`{"hits":[]}`), Ext: "json"},
			{},
			{Data: []byte("<rss/>"), Ext: "xml"},
		},
	}

	deniedKey := "raw/denied/2026-10-14/" + run.ID + ".xml"
	fake := &fakeS3{objects: map[string][]byte{}, types: map[string]string{}, fail: map[string]bool{deniedKey: true}}
	a := &archive{bucket: "test-bucket", client: fake}

	err := a.save(context.Background(), result, news.IngestResult{Inserted: 2, DuplicatesByURL: 1, DuplicatesByTitle: 1}, started.Add(5*time.Second))
	if err == nil {
		t.Fatal("save returned nil, want the denied source's error")
	}

	if got := fake.types["raw/cncf/2026-10-14/"+run.ID+".xml"]; got != "application/xml" {
		t.Errorf("cncf payload content type = %q, want application/xml", got)
	}
	if got := string(fake.objects["raw/hackernews/2026-10-14/"+run.ID+".json"]); got != `{"hits":[]}` {
		t.Errorf("hackernews payload = %q", got)
	}
	if len(fake.objects) != 3 {
		t.Errorf("wrote %d objects, want 2 payloads and the manifest", len(fake.objects))
	}

	// The manifest is written despite the failed payload, and records where
	// each stored payload is.
	var m collector.Manifest
	if err := json.Unmarshal(fake.objects["runs/"+run.ID+".json"], &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if m.ArticlesWritten != 2 || m.DuplicatesByURL != 1 || m.DuplicatesByTitle != 1 {
		t.Errorf("manifest counts = %+v", m)
	}

	paths := map[string]string{}
	for _, sr := range m.Sources {
		paths[sr.Name] = sr.RawPath
	}
	if paths["cncf"] != "raw/cncf/2026-10-14/"+run.ID+".xml" || paths["broken"] != "" || paths["denied"] != "" {
		t.Errorf("manifest raw paths = %v", paths)
	}

	// The caller's result is left as it was.
	if result.Sources[0].RawPath != "" {
		t.Errorf("save modified the caller's sources")
	}
}
