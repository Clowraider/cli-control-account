package ego

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTodayHTTP(t *testing.T) {
	s, err := OpenStorage(filepath.Join(t.TempDir(), "today.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewHandler(NewWorker(s, 1, 1, time.Second))
	const start int64 = 1741457700000
	const end = start + 23*60*60*1000
	var events []EgoEvent
	for _, ts := range []int64{start - 1, start, end - 1, end} {
		events = append(events, EgoEvent{Timestamp: ts, Provider: "openai", Model: "gpt-4o", Account: "account", Status: "success", TotalTokens: 1})
	}
	if err := s.InsertBatch(events); err != nil {
		t.Fatal(err)
	}
	get := func(query string, dst any) int {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", "/ego/api/"+query, nil))
		if dst != nil && json.Unmarshal(r.Body.Bytes(), dst) != nil {
			t.Fatalf("invalid JSON: %s", r.Body.String())
		}
		return r.Code
	}
	bounds := fmt.Sprintf("?range=today&start=%d&end=%d", start, end)
	var stats SummaryStats
	var timeline []TimelinePoint
	switch {
	case get("stats"+bounds, &stats) != 200 || stats.TotalRequests != 2:
		t.Fatalf("stats: %+v", stats)
	case get("timeline"+bounds, &timeline) != 200 || len(timeline) == 0 || timeline[0].Timestamp != start:
		t.Fatalf("timeline: %+v", timeline)
	case get("stats?range=today&start=2&end=1", nil) != 400:
		t.Fatal("invalid bounds must return 400")
	}
}
