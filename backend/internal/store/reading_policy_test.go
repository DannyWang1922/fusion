package store

import (
	"testing"
	"time"
)

func TestReadingPolicyPreservesHiddenItemsAndReadState(t *testing.T) {
	s, _ := setupTestDB(t)
	defer closeStore(t, s)
	group := mustCreateGroup(t, s, "Policy")
	feed := mustCreateFeed(t, s, group.ID, "F", "https://openai.com/news/rss.xml", "", "")
	now := time.Now().Unix()
	inputs := []BatchCreateItemInput{
		{GUID: "selected", Title: "policy selected", PubDate: now - 3600, Categories: []string{"Research"}},
		{GUID: "old", Title: "policy old", PubDate: now - 91*86400, Categories: []string{"Research"}},
		{GUID: "other", Title: "policy other", PubDate: now - 3600, Categories: []string{"Company"}},
		{GUID: "untagged", Title: "policy untagged", PubDate: now - 3600},
		{GUID: "future", Title: "policy future", PubDate: now + 3600, Categories: []string{"Research"}},
	}
	if n, err := s.BatchCreateItemsIgnore(feed.ID, inputs); err != nil || n != 5 {
		t.Fatalf("initial ingest: %d %v", n, err)
	}
	rows, _ := s.ListItems(ListItemsParams{FeedID: &feed.ID})
	for _, item := range rows {
		if item.GUID == "other" {
			if err := s.UpdateItemUnread(item.ID, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	allowed := []string{"Research", "Publication"}
	days := int64(90)
	if err := s.UpdateFeed(feed.ID, UpdateFeedParams{AllowedCategories: &allowed, MaxAgeDays: &days}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListItems(ListItemsParams{FeedID: &feed.ID})
	if err != nil || len(rows) != 1 || rows[0].GUID != "selected" {
		t.Fatalf("visible list: %+v %v", rows, err)
	}
	if count, err := s.CountItems(ListItemsParams{GroupID: &group.ID}); err != nil || count != 1 {
		t.Fatalf("count: %d %v", count, err)
	}
	feeds, err := s.ListFeeds()
	if err != nil || feeds[0].ItemCount != 1 || feeds[0].UnreadCount != 1 {
		t.Fatalf("feed counts: %+v %v", feeds, err)
	}
	search, err := s.SearchItems("policy", 100)
	if err != nil || len(search) != 1 {
		t.Fatalf("search: %+v %v", search, err)
	}
	ids, err := s.ListUnreadItemIDs()
	if err != nil || len(ids) != 1 {
		t.Fatalf("unread IDs: %v %v", ids, err)
	}
	fever, err := s.ListFeverItems(ListFeverItemsParams{})
	if err != nil || len(fever) != 1 {
		t.Fatalf("fever: %v %v", fever, err)
	}
	if err := s.MarkAllAsRead(nil); err != nil {
		t.Fatal(err)
	}
	// Existing metadata can be backfilled, but out-of-scope new rows are skipped.
	inputs = append(inputs, BatchCreateItemInput{GUID: "new-rejected", PubDate: now, Categories: []string{"Company"}}, BatchCreateItemInput{GUID: "new-selected", PubDate: now, Categories: []string{"Publication"}})
	if n, err := s.BatchCreateItemsIgnore(feed.ID, inputs); err != nil || n != 1 {
		t.Fatalf("filtered ingest: %d %v", n, err)
	}
	empty := []string{}
	zero := int64(0)
	if err := s.UpdateFeed(feed.ID, UpdateFeedParams{AllowedCategories: &empty, MaxAgeDays: &zero}); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ListItems(ListItemsParams{FeedID: &feed.ID})
	if err != nil || len(rows) != 6 {
		t.Fatalf("restore rows: %d %v", len(rows), err)
	}
	for _, item := range rows {
		if item.GUID == "other" && item.Unread {
			t.Error("read state replaced")
		}
		if item.GUID == "old" && !item.Unread {
			t.Error("hidden item was marked read")
		}
		if item.GUID == "new-rejected" {
			t.Error("rejected new item was inserted")
		}
	}
}

func TestFullContentCacheDoesNotReplaceSummary(t *testing.T) {
	s, _ := setupTestDB(t)
	defer closeStore(t, s)
	g := mustCreateGroup(t, s, "Cache")
	f := mustCreateFeed(t, s, g.ID, "F", "https://openai.com/news/rss.xml", "", "")
	item := mustCreateItem(t, s, f.ID, "g", "Title", "https://openai.com/index/test/", "RSS summary", time.Now().Unix())
	body := "<p>Full article.</p>"
	if err := s.SaveFullContent(item.ID, item.Link, body); err != nil {
		t.Fatal(err)
	}
	detail, err := s.GetItem(item.ID)
	if err != nil || detail.Content != body || detail.FullContentFetchedAt == 0 {
		t.Fatalf("cache overlay: %+v %v", detail, err)
	}
	list, err := s.ListItems(ListItemsParams{})
	if err != nil || list[0].Content != "RSS summary" {
		t.Fatalf("summary changed: %+v %v", list, err)
	}
	if err := s.DeleteFeed(f.ID); err != nil {
		t.Fatal(err)
	}
	if cache, err := s.GetFullContent(item.ID); err != nil || cache != nil {
		t.Fatalf("cache was not deleted: %+v %v", cache, err)
	}
}
