package store

import (
	"database/sql"
	"errors"
)

// FullContent is a cached article body, separate from the original RSS summary.
type FullContent struct {
	SourceURL   string `json:"source_url"`
	ContentHTML string `json:"content_html"`
	FetchedAt   int64  `json:"fetched_at"`
}

func (s *Store) GetFullContent(itemID int64) (*FullContent, error) {
	cache := &FullContent{}
	err := s.db.QueryRow(`SELECT source_url,content_html,fetched_at FROM item_full_content WHERE item_id=:id`, sql.Named("id", itemID)).Scan(&cache.SourceURL, &cache.ContentHTML, &cache.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (s *Store) SaveFullContent(itemID int64, sourceURL, content string) error {
	_, err := s.db.Exec(`INSERT INTO item_full_content(item_id,source_url,content_html) VALUES(:id,:source_url,:content)
 ON CONFLICT(item_id) DO UPDATE SET source_url=excluded.source_url,content_html=excluded.content_html,fetched_at=unixepoch()`, sql.Named("id", itemID), sql.Named("source_url", sourceURL), sql.Named("content", content))
	return err
}
