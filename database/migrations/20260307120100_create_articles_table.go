package migrations

import (
	"database/sql"
	"log"
)

// Up 创建文章表
func Up_20260307120100_create_articles_table(db *sql.DB) error {
	log.Println("Creating articles table...")

	query := `
		CREATE TABLE IF NOT EXISTS articles (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(200) NOT NULL,
			slug VARCHAR(200) UNIQUE,
			content TEXT,
			summary VARCHAR(500),
			cover_image VARCHAR(255),
			author_id BIGINT NOT NULL,
			category_id BIGINT,
			status TINYINT DEFAULT 1,
			view_count INT DEFAULT 0,
			like_count INT DEFAULT 0,
			comment_count INT DEFAULT 0,
			published_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL,
			INDEX idx_author (author_id),
			INDEX idx_category (category_id),
			INDEX idx_status (status),
			INDEX idx_published (published_at),
			FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	log.Println("Articles table created successfully")
	return nil
}

// Down 删除文章表
func Down_20260307120100_create_articles_table(db *sql.DB) error {
	log.Println("Dropping articles table...")

	_, err := db.Exec("DROP TABLE IF EXISTS articles")
	if err != nil {
		return err
	}

	log.Println("Articles table dropped successfully")
	return nil
}
