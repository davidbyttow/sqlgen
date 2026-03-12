-- Test fixture: a realistic MySQL schema with various column types, FKs, views, and a join table.

CREATE TABLE organizations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_org_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    org_id BIGINT UNSIGNED NOT NULL,
    email VARCHAR(255) NOT NULL,
    role ENUM('admin', 'member', 'guest') NOT NULL DEFAULT 'member',
    `name` VARCHAR(255),
    metadata JSON,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_email (email),
    CONSTRAINT fk_user_org FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE posts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    author_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(500) NOT NULL,
    body TEXT NOT NULL,
    published TINYINT(1) NOT NULL DEFAULT 0,
    view_count INT UNSIGNED NOT NULL DEFAULT 0,
    score DECIMAL(10,2),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (author_id) REFERENCES users(id)
) ENGINE=InnoDB;

CREATE TABLE tags (
    id INT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(100) NOT NULL UNIQUE,
    PRIMARY KEY (id)
);

-- Many-to-many join table
CREATE TABLE post_tags (
    post_id BIGINT UNSIGNED NOT NULL,
    tag_id INT NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE VIEW published_posts AS
    SELECT id, title, author_id FROM posts WHERE published = 1;

-- Table with various numeric types
CREATE TABLE type_examples (
    id INT AUTO_INCREMENT PRIMARY KEY,
    tiny_val TINYINT,
    small_val SMALLINT,
    medium_val MEDIUMINT,
    big_val BIGINT,
    float_val FLOAT,
    double_val DOUBLE,
    decimal_val DECIMAL(10,2),
    bool_val BOOLEAN,
    date_val DATE,
    time_val TIME,
    datetime_val DATETIME,
    timestamp_val TIMESTAMP,
    year_val YEAR,
    char_val CHAR(10),
    varchar_val VARCHAR(255),
    text_val TEXT,
    mediumtext_val MEDIUMTEXT,
    longtext_val LONGTEXT,
    blob_val BLOB,
    json_val JSON,
    binary_val BINARY(16)
);

-- Self-referencing FK
CREATE TABLE categories (
    id INT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(255) NOT NULL,
    parent_id INT,
    PRIMARY KEY (id),
    FOREIGN KEY (parent_id) REFERENCES categories(id)
);

-- Audit log with ALTER TABLE FK
CREATE TABLE audit_log (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    action VARCHAR(100) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE audit_log ADD CONSTRAINT fk_audit_user
    FOREIGN KEY (user_id) REFERENCES users(id);

-- Index
CREATE UNIQUE INDEX idx_users_email_org ON users (email, org_id);
CREATE INDEX idx_posts_author ON posts (author_id);
