package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0x2E/fusion/internal/store"
	"github.com/0x2E/fusion/internal/translation"
)

func TestFullContentJobDedupRetryCacheAndTranslationSource(t *testing.T) {
	h, st := newFeverTestHandler(t)
	defer h.CloseFullContent()
	g, _ := st.CreateGroup("Full text")
	f, _ := st.CreateFeed(g.ID, "OpenAI", "https://openai.com/news/rss.xml", "", "", nil)
	enabled := true
	if err := st.UpdateFeed(f.ID, store.UpdateFeedParams{FetchFullContent: &enabled}); err != nil {
		t.Fatal(err)
	}
	item, _ := st.CreateItem(f.ID, "full", "Title", "https://openai.com/index/test/", "<p>RSS summary.</p>", time.Now().Unix())
	oldKey := h.translationSourceKey(item)
	var calls atomic.Int32
	gate := make(chan struct{})
	body := "<h2>Research findings</h2><p>" + strings.Repeat("Original article paragraph. ", 20) + "</p>"
	h.fetchFullContent = func(ctx context.Context, raw string) (string, error) {
		attempt := calls.Add(1)
		select {
		case <-gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if attempt == 1 {
			return "", errors.New("temporary browser failure")
		}
		return body, nil
	}
	r := newTestRouter()
	r.GET("/items/:id/full-content", h.getFullContent)
	r.POST("/items/:id/full-content", h.startFullContent)
	path := "/items/" + strconv.FormatInt(item.ID, 10) + "/full-content"
	post := func() {
		t.Helper()
		w := performRequest(r, http.MethodPost, path, nil, nil)
		if w.Code != http.StatusAccepted && w.Code != http.StatusOK {
			t.Fatalf("POST %d %s", w.Code, w.Body.String())
		}
	}
	status := func() fullContentResponse {
		t.Helper()
		w := performRequest(r, http.MethodGet, path, nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %d", w.Code)
		}
		var response struct {
			Data fullContentResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Data
	}
	wait := func(expected string) fullContentResponse {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			response := status()
			if response.State == expected {
				return response
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("job never became %s", expected)
		return fullContentResponse{}
	}
	post()
	post()
	close(gate)
	wait("error")
	if calls.Load() != 1 {
		t.Fatalf("duplicate workers: %d", calls.Load())
	}
	post()
	result := wait("idle")
	if result.Item == nil || result.Item.Content != body {
		t.Fatalf("body not returned: %+v", result)
	}
	post()
	if calls.Load() != 2 {
		t.Fatalf("cache refetched: %d", calls.Load())
	}
	cached, _ := st.GetItem(item.ID)
	if oldKey == h.translationSourceKey(cached) {
		t.Error("translation source did not change from summary to full body")
	}
	if !validateTranslationSegments(cached.Content, []translation.Segment{{ID: "0", Text: "Research findings"}}) {
		t.Error("full article rejected by translation")
	}
}
