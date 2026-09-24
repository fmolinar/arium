package collector

import "context"

// Source is a single upstream the collector pulls from.
type Source interface {
	// Name identifies the source in storage paths and run manifests. It must
	// be a valid storage name (see ValidName).
	Name() string

	// Fetch returns the raw upstream payload, kept verbatim on disk, together
	// with the articles parsed out of it. Articles do not need to be
	// normalized; the collector does that.
	Fetch(ctx context.Context) (raw []byte, articles []Article, err error)
}
