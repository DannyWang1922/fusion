package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/0x2E/fusion/internal/translation"
)

func (s *Store) GetTranslation(itemID int64, sourceKey string) ([]translation.Segment, error) {
	var data string
	err := s.db.QueryRow(`SELECT segments_json FROM article_translations WHERE item_id=:item_id AND source_key=:source_key`,
		sql.Named("item_id", itemID), sql.Named("source_key", sourceKey)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return []translation.Segment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get translation: %w", err)
	}
	var segments []translation.Segment
	if err := json.Unmarshal([]byte(data), &segments); err != nil {
		return nil, err
	}
	return segments, nil
}

func (s *Store) SaveTranslation(itemID int64, sourceKey string, segments []translation.Segment) error {
	data, err := json.Marshal(segments)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO article_translations (item_id,source_key,segments_json)
 VALUES (:item_id,:source_key,:segments_json)
 ON CONFLICT(item_id) DO UPDATE SET source_key=excluded.source_key,segments_json=excluded.segments_json,updated_at=unixepoch()`,
		sql.Named("item_id", itemID), sql.Named("source_key", sourceKey), sql.Named("segments_json", string(data)))
	return err
}
