-- The admin page's content: drafts per collection and language, and the
-- releases published to S3 for the app. data holds the item as a JSON object
-- (DSQL has no json type, so it is text).

CREATE TABLE IF NOT EXISTS content_items (
    collection text        NOT NULL,
    lang       text        NOT NULL,
    item_id    text        NOT NULL,
    position   int         NOT NULL,
    data       text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by text        NOT NULL,
    PRIMARY KEY (collection, lang, item_id)
);

CREATE TABLE IF NOT EXISTS content_releases (
    version      int         PRIMARY KEY,
    published_at timestamptz NOT NULL,
    published_by text        NOT NULL,
    items        int         NOT NULL
);

GRANT SELECT, INSERT, UPDATE, DELETE ON content_items, content_releases TO missale_api;
