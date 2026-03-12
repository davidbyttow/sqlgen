-- SQLite test schema for parser tests.

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT,
    age INTEGER NOT NULL DEFAULT 0,
    bio TEXT DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT,
    published BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tags (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- Join table for many-to-many.
CREATE TABLE post_tags (
    post_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE comments (
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT fk_comment_post FOREIGN KEY (post_id) REFERENCES posts(id),
    CONSTRAINT fk_comment_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_users_email ON users (email);
CREATE INDEX idx_posts_user_id ON posts (user_id);
CREATE UNIQUE INDEX idx_tags_name ON tags (name);

CREATE VIEW published_posts AS
    SELECT id, title, body FROM posts WHERE published = 1;
