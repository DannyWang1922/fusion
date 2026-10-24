package store

import (
	"database/sql"
	"testing"

	"github.com/0x2E/fusion/internal/translation"
)

func TestTranslationCacheInvalidationAndDeletion(t *testing.T) {
	st, _ := setupTestDB(t)
	defer closeStore(t, st)
	group := mustCreateGroup(t, st, "G")
	feed := mustCreateFeed(t, st, group.ID, "F", "https://example.com/rss", "https://example.com", "")
	item := mustCreateItem(t, st, feed.ID, "id", "Title", "https://example.com/1", "<p>Original</p>", 1)
	saved := []translation.Segment{{ID: "0", Text: "Original", Translation: "原文"}}
	if err := st.SaveTranslation(item.ID, "source-v1", saved); err != nil {
		t.Fatal(err)
	}
	found, err := st.GetTranslation(item.ID, "source-v1")
	if err != nil || len(found) != 1 || found[0].Translation != "原文" {
		t.Fatalf("cache miss: %v %v", found, err)
	}
	stale, err := st.GetTranslation(item.ID, "source-v2")
	if err != nil || len(stale) != 0 {
		t.Fatalf("stale cache leaked: %v %v", stale, err)
	}
	if _, err := st.db.Exec("DELETE FROM items WHERE id=:id", sql.Named("id", item.ID)); err != nil {
		t.Fatal(err)
	}
	found, err = st.GetTranslation(item.ID, "source-v1")
	if err != nil || len(found) != 0 {
		t.Fatalf("deleted article translation remains: %v %v", found, err)
	}
}
