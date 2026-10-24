ALTER TABLE feeds ADD COLUMN allowed_categories TEXT NOT NULL DEFAULT '[]';
ALTER TABLE feeds ADD COLUMN max_age_days INTEGER NOT NULL DEFAULT 0 CHECK(max_age_days BETWEEN 0 AND 3650);
ALTER TABLE feeds ADD COLUMN fetch_full_content INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN categories TEXT NOT NULL DEFAULT '[]';

CREATE VIEW visible_items AS
SELECT i.* FROM items i JOIN feeds f ON f.id = i.feed_id
WHERE (f.max_age_days = 0 OR (i.pub_date >= unixepoch() - f.max_age_days * 86400 AND i.pub_date <= unixepoch()))
AND (json_array_length(f.allowed_categories) = 0 OR EXISTS (
 SELECT 1 FROM json_each(i.categories) c JOIN json_each(f.allowed_categories) a ON c.value = a.value
));

CREATE TABLE item_full_content (
 item_id INTEGER PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
 source_url TEXT NOT NULL,
 content_html TEXT NOT NULL,
 fetched_at INTEGER NOT NULL DEFAULT (unixepoch())
);
