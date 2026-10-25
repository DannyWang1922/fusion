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
	if err := s.UpdateFeed(feed.ID, UpdateFeedParams{AllowedCategories: &allowed}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListItems(ListItemsParams{FeedID: &feed.ID, Period: "recent"})
	if err != nil || len(rows) != 1 || rows[0].GUID != "selected" {
		t.Fatalf("visible list: %+v %v", rows, err)
	}
	if count, err := s.CountItems(ListItemsParams{GroupID: &group.ID, Period: "recent"}); err != nil || count != 1 {
		t.Fatalf("count: %d %v", count, err)
	}
	feeds, err := s.ListFeeds()
	if err != nil || feeds[0].ItemCount != 1 || feeds[0].UnreadCount != 1 || feeds[0].OlderItemCount != 1 || feeds[0].OlderUnreadCount != 1 {
		t.Fatalf("feed counts: %+v %v", feeds, err)
	}
	search, err := s.SearchItemsInPeriod("policy", 100, "recent")
	if err != nil || len(search) != 1 {
		t.Fatalf("search: %+v %v", search, err)
	}
	ids, err := s.ListUnreadItemIDs()
	if err != nil || len(ids) != 3 {
		t.Fatalf("unread IDs: %v %v", ids, err)
	}
	fever, err := s.ListFeverItems(ListFeverItemsParams{})
	if err != nil || len(fever) != 3 {
		t.Fatalf("fever: %v %v", fever, err)
	}
	if err := s.BatchUpdateItemsUnread([]int64{rows[0].ID}, false); err != nil {
		t.Fatal(err)
	}
	// Category restrictions still apply, while matching historical rows are retained.
	inputs = append(inputs, BatchCreateItemInput{GUID: "new-rejected", PubDate: now, Categories: []string{"Company"}}, BatchCreateItemInput{GUID: "new-selected", PubDate: now, Categories: []string{"Publication"}}, BatchCreateItemInput{GUID: "new-old", PubDate: now - 120*86400, Categories: []string{"Research"}})
	if n, err := s.BatchCreateItemsIgnore(feed.ID, inputs); err != nil || n != 2 {
		t.Fatalf("filtered ingest: %d %v", n, err)
	}
	empty := []string{}
	if err := s.UpdateFeed(feed.ID, UpdateFeedParams{AllowedCategories: &empty}); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ListItems(ListItemsParams{FeedID: &feed.ID})
	if err != nil || len(rows) != 7 {
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
