package ego

import (
	"encoding/json"
	"fmt"
	"math"
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
	const start int64 = 1741457700000 // Deliberately not aligned to a UTC hour.
	const end = start + 23*60*60*1000
	var events []EgoEvent
	for _, ts := range []int64{start - 1, start, start + 3600000 + 1, end - 1, end} {
		events = append(events, EgoEvent{
			Timestamp: ts, Provider: "openai", Model: "gpt-4o", Account: "account",
			Status: "success", PromptTokens: 1000000, CompletionTokens: 1000000, TotalTokens: 2000000,
		})
	}
	events = append(events, EgoEvent{
		Timestamp: start, Provider: "other", Model: "gpt-4o", Account: "", Status: "success", TotalTokens: 7,
	})
	if err := s.InsertBatch(events); err != nil {
		t.Fatal(err)
	}
	get := func(endpoint, query string) (int, string) {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", "/ego/api/"+endpoint+query, nil))
		return r.Code, r.Body.String()
	}
	endpoints := []string{"stats", "timeline", "providers", "models", "accounts"}
	query := fmt.Sprintf("?range=today&start=%d&end=%d&provider=openai", start, end)
	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			status, body := get(endpoint, query)
			if status != 200 {
				t.Fatalf("status %d: %s", status, body)
			}
			var rows []map[string]any
			if endpoint == "stats" {
				var row map[string]any
				if err := json.Unmarshal([]byte(body), &row); err != nil {
					t.Fatal(err)
				}
				rows = []map[string]any{row}
			} else if err := json.Unmarshal([]byte(body), &rows); err != nil {
				t.Fatal(err)
			}
			var requests, tokens float64
			for _, row := range rows {
				requests += row["total_requests"].(float64)
				tokens += row["total_tokens"].(float64)
			}
			wantRequests, wantTokens := float64(3), float64(6000000)
			if endpoint == "providers" {
				wantRequests++ // Provider rankings intentionally ignore the provider filter.
				wantTokens += 7
			}
			if requests != wantRequests || tokens != wantTokens {
				t.Fatalf("requests/tokens: %s", body)
			}
			if endpoint == "timeline" {
				for i, row := range rows {
					want := start + int64(i)*3600000
					if i == 2 {
						want = end - 3600000
					}
					if row["timestamp"] != float64(want) {
						t.Fatalf("bucket start: %s", body)
					}
				}
			}
			if endpoint == "stats" || endpoint == "models" {
				pricing, _ := FindModelPricing(s.pricingCache, "gpt-4o")
				cost, _, _ := CalculateTokenCost(pricing, "openai", 3000000, 3000000, 0, 0, 0)
				if rows[0]["estimated_cost_usd"] != math.Round(cost*10000)/10000 {
					t.Fatalf("bounded cost: want %v, got %s", cost, body)
				}
			}
			_, empty := get(endpoint, fmt.Sprintf("?range=today&start=%d&end=%d", end+1, end+3600001))
			if endpoint == "stats" {
				var row SummaryStats
				if err := json.Unmarshal([]byte(empty), &row); err != nil {
					t.Fatal(err)
				}
				if row.TotalRequests != 0 {
					t.Fatalf("not empty: %s", empty)
				}
			} else if empty != "[]\n" {
				t.Fatalf("not empty: %s", empty)
			}
		})
	}
	for _, endpoint := range endpoints {
		for _, bounds := range []string{
			"", "&start=NaN&end=2", "&start=1.5&end=2", "&start=0&end=1",
			"&start=-1&end=1", "&start=2&end=1", "&start=1&end=93600002",
			"&start=9223372036854775808&end=2", "&start=1&end=Infinity",
		} {
			status, body := get(endpoint, "?range=today"+bounds)
			const want = "{\"error\":true,\"message\":\"invalid Today bounds: start and end must be positive epoch milliseconds, ordered and at most 26 hours apart\"}\n"
			if status != 400 || body != want {
				t.Errorf("%s%s: %d %s", endpoint, bounds, status, body)
			}
		}
		for _, r := range []string{"", "1h", "24h", "7d", "30d", "all", "unknown"} {
			status, body := get(endpoint, "?range="+r)
			status2, body2 := get(endpoint, "?range="+r+"&start=bad&end=bad")
			if status != status2 || body != body2 {
				t.Errorf("legacy changed: %s %s", endpoint, r)
			}
		}
	}
	_, accounts := get("accounts", fmt.Sprintf("?range=today&start=%d&end=%d", start, end))
	var rows []AccountRanking
	if err := json.Unmarshal([]byte(accounts), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Account != "account" {
		t.Fatalf("blank account must be excluded: %s", accounts)
	}
}
