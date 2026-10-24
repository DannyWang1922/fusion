DROP VIEW visible_items;
ALTER TABLE feeds DROP COLUMN max_age_days;

CREATE VIEW eligible_items AS
SELECT i.* FROM items i JOIN feeds f ON f.id=i.feed_id
WHERE json_array_length(f.allowed_categories)=0 OR EXISTS (
 SELECT 1 FROM json_each(i.categories) c JOIN json_each(f.allowed_categories) a ON c.value=a.value
);

CREATE VIEW visible_items AS
SELECT * FROM eligible_items
WHERE pub_date>=unixepoch()-90*86400 AND pub_date<=unixepoch();

-- Backfill metadata for historical entries that the previous age policy skipped.
UPDATE feed_fetch_state SET etag='',last_modified='',next_check_at=unixepoch(),retry_after_until=0;
