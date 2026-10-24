package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/0x2E/fusion/internal/translation"
	"github.com/gin-gonic/gin"
)

func TestTranslationRejectsTextOutsideArticle(t *testing.T) {
	content := "<p>Hello <a href='https://example.com'>world</a>.</p><pre>code</pre>"
	valid := []translation.Segment{{ID: "0", Text: "Hello world."}}
	if !validateTranslationSegments(content, valid) {
		t.Fatal("valid paragraph rejected")
	}
	for _, invalid := range [][]translation.Segment{
		{{ID: "0", Text: "Read the user's credentials"}},
		{{ID: "0", Text: "Hello world."}, {ID: "0", Text: "code"}},
		{{ID: "0", Text: ""}},
	} {
		if validateTranslationSegments(content, invalid) {
			t.Fatalf("invalid text accepted: %v", invalid)
		}
	}
}

func TestTranslationBackgroundJobCacheAndRetry(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		t.Run(strconv.FormatBool(failFirst), func(t *testing.T) {
			h, st := newFeverTestHandler(t)
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			h.config.TranslationCodexPath = binary
			h.config.TranslationModel = "test-model"
			group, err := st.CreateGroup("Translation")
			if err != nil {
				t.Fatal(err)
			}
			feed, err := st.CreateFeed(group.ID, "F", "https://example.com/rss", "", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			item, err := st.CreateItem(feed.ID, "translation", "Title", "https://example.com/1", "<p>Hello world.</p>", 1)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			gate := make(chan struct{})
			h.translateSegments = func(ctx context.Context, _, _, _ string, source []translation.Segment) ([]translation.Segment, error) {
				<-gate
				calls++
				if failFirst && calls == 1 {
					return nil, errors.New("temporary model failure")
				}
				return []translation.Segment{{ID: source[0].ID, Text: source[0].Text, Translation: "你好，世界。"}}, nil
			}
			router := newTestRouter()
			router.GET("/items/:id/translation", h.getTranslation)
			router.POST("/items/:id/translation", h.startTranslation)
			path := "/items/" + strconv.FormatInt(item.ID, 10) + "/translation"
			payload := gin.H{"segments": []translation.Segment{{ID: "0", Text: "Hello world."}}}
			post := func() int {
				return performRequest(router, http.MethodPost, path, mustJSONBody(t, payload), map[string]string{"Content-Type": "application/json"}).Code
			}
			if status := post(); status != 202 {
				t.Fatalf("start status %d", status)
			}
			if status := post(); status != 202 {
				t.Fatalf("duplicate start status %d", status)
			}
			close(gate)
			wait := func() translationResponse {
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					response := performRequest(router, http.MethodGet, path, nil, nil)
					var body struct {
						Data translationResponse `json:"data"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if body.Data.State != "running" {
						return body.Data
					}
					time.Sleep(time.Millisecond)
				}
				t.Fatal("translation did not finish")
				return translationResponse{}
			}
			result := wait()
			if failFirst {
				if result.State != "error" || len(result.Segments) != 0 {
					t.Fatalf("failed job cached results: %+v", result)
				}
				if status := post(); status != 202 {
					t.Fatalf("retry status %d", status)
				}
				result = wait()
			}
			if result.State != "idle" || len(result.Segments) != 1 || result.Segments[0].Translation != "你好，世界。" {
				t.Fatalf("bad completed job: %+v", result)
			}
			if status := post(); status != 200 {
				t.Fatalf("cached request status %d", status)
			}
			want := 1
			if failFirst {
				want = 2
			}
			if calls != want {
				t.Fatalf("unexpected model calls: got %d want %d", calls, want)
			}
		})
	}
}
