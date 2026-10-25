package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/store"
)

func TestRecentDefaultAndOlderWindowPreserveSavedContent(t *testing.T) {
	h, st := newFeverTestHandler(t)
	group, _ := st.CreateGroup("G")
	feed, _ := st.CreateFeed(group.ID, "Blog", "https://example.com/feed", "", "", nil)
	now := time.Now().Unix()
	// Ingestion must retain old entries for the historical list.
	if n, err := st.BatchCreateItemsIgnore(feed.ID, []store.BatchCreateItemInput{
		{GUID: "new", Title: "window recent", PubDate: now - 3600},
		{GUID: "old", Title: "window older", PubDate: now - 120*86400},
		{GUID: "older", Title: "window oldest", PubDate: now - 200*86400},
	}); err != nil || n != 3 {
		t.Fatalf("ingest: %d %v", n, err)
	}
	router := newTestRouter()
	router.GET("/api/items", h.listItems)
	router.GET("/api/search", h.search)
	type response struct {
		Data       []model.Item `json:"data"`
		Total      int          `json:"total"`
		OlderTotal int          `json:"older_total"`
		Next       *string      `json:"next_cursor"`
	}
	read := func(path string) response {
		t.Helper()
		w := performRequest(router, http.MethodGet, path, nil, nil)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var result response
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	recent := read("/api/items")
	if len(recent.Data) != 1 || recent.Data[0].GUID != "new" || recent.OlderTotal != 2 {
		t.Fatalf("default response: %+v", recent)
	}
	old := read("/api/items?period=older&limit=1")
	if len(old.Data) != 1 || old.Data[0].GUID != "old" || old.Total != 2 || old.Next == nil {
		t.Fatalf("older first page: %+v", old)
	}
	next := read("/api/items?period=older&limit=1&before=" + *old.Next)
	if len(next.Data) != 1 || next.Data[0].GUID != "older" {
		t.Fatalf("older next page: %+v", next)
	}
	// Merely viewing older items neither marks them read nor changes recent badges.
	for _, item := range old.Data {
		actual, _ := st.GetItem(item.ID)
		if !actual.Unread {
			t.Error("view changed read state")
		}
	}
	feeds, _ := st.ListFeeds()
	if feeds[0].UnreadCount != 1 || feeds[0].ItemCount != 1 || feeds[0].OlderUnreadCount != 2 || feeds[0].OlderItemCount != 2 {
		t.Fatalf("recent counts: %+v", feeds[0])
	}
	item := old.Data[0]
	if _, err := st.CreateBookmark(&item.ID, &feed.ID, "https://example.com/old", item.Title, "<p>Saved</p>", item.PubDate, feed.Name); err != nil {
		t.Fatal(err)
	}
	bookmarks, err := st.ListBookmarks(store.ListBookmarksParams{})
	if err != nil || len(bookmarks) != 1 {
		t.Fatalf("historical bookmark: %v %v", bookmarks, err)
	}
	if err := st.BatchUpdateItemsUnread([]int64{item.ID}, false); err != nil {
		t.Fatal(err)
	}
	feeds, _ = st.ListFeeds()
	if feeds[0].UnreadCount != 1 || feeds[0].OlderUnreadCount != 1 || feeds[0].OlderItemCount != 2 {
		t.Fatal("historical read did not update only its own unread window")
	}
	if err := st.BatchUpdateItemsUnread([]int64{recent.Data[0].ID}, false); err != nil {
		t.Fatal(err)
	}
	feeds, _ = st.ListFeeds()
	if feeds[0].UnreadCount != 0 || feeds[0].OlderUnreadCount != 1 {
		t.Fatalf("recent read changed historical count: %+v", feeds[0])
	}
	for _, query := range []struct {
		path string
		want int
	}{{"/api/search?q=window", 1}, {"/api/search?q=window&period=all", 3}} {
		w := performRequest(router, http.MethodGet, query.path, nil, nil)
		var result struct {
			Data struct {
				Items []store.SearchItemResult `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data.Items) != query.want {
			t.Fatalf("search %s: %d", query.path, len(result.Data.Items))
		}
	}
	w := performRequest(router, http.MethodGet, "/api/items?period=invalid", nil, nil)
	if w.Code != 400 {
		t.Fatalf("invalid period accepted: %d", w.Code)
	}
}
