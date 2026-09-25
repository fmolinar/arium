package news

import (
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	want := Cursor{PublishedAt: time.Date(2026, 9, 24, 12, 30, 0, 123, time.UTC), ID: "0123456789abcdef"}

	got, err := DecodeCursor(EncodeCursor(want))
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if !got.PublishedAt.Equal(want.PublishedAt) || got.ID != want.ID {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, token := range []string{"!!!", encodeRaw("no-separator"), encodeRaw("not-a-time|abc"), encodeRaw("2026-09-24T00:00:00Z|")} {
		if _, err := DecodeCursor(token); err == nil {
			t.Errorf("DecodeCursor(%q) = nil error, want ErrInvalidCursor", token)
		}
	}
}

func TestParseListQuery(t *testing.T) {
	cursor := EncodeCursor(Cursor{PublishedAt: time.Now().UTC(), ID: "abc"})

	tests := []struct {
		query     string
		wantErr   bool
		wantTag   string
		wantLimit int
		wantAfter bool
	}{
		{"", false, "", defaultLimit, false},
		{"tag=sre&limit=5", false, "sre", 5, false},
		{"cursor=" + cursor, false, "", defaultLimit, true},
		{"tag=frontend", true, "", 0, false},
		{"limit=0", true, "", 0, false},
		{"limit=101", true, "", 0, false},
		{"limit=ten", true, "", 0, false},
		{"cursor=bogus", true, "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			q, msg := parseListQuery(httptest.NewRequest("GET", "/api/v1/news?"+tt.query, nil))
			if (msg != "") != tt.wantErr {
				t.Fatalf("error = %q, wantErr %v", msg, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if q.Tag != tt.wantTag || q.Limit != tt.wantLimit || (q.After != nil) != tt.wantAfter {
				t.Errorf("query = %+v, want tag %q limit %d after %v", q, tt.wantTag, tt.wantLimit, tt.wantAfter)
			}
		})
	}
}

// encodeRaw encodes an arbitrary cursor payload, for building malformed tokens.
func encodeRaw(raw string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
