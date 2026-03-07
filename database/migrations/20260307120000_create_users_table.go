package migrations

import (
	"database/sql"
	"log"
)

// Up 创建用户表
func Up_20260307120000_create_users_table(db *sql.DB) error {
	log.Println("Creating users table...")
	
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(50) NOT NULL UNIQUE,
			email VARCHAR(100) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			role VARCHAR(20) DEFAULT 'user',
			status TINYINT DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL,
			INDEX idx_username (username),
			INDEX idx_email (email),
			INDEX idx_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
	
	_, err := db.Exec(query)
	if err != nil {
		return err
	}
	
	log.Println("Users table created successfully")
	return nil
}

// Down 删除用户表
func Down_20260307120000_create_users_table(db *sql.DB) error {
	log.Println("Dropping users table...")
	
	_, err := db.Exec("DROP TABLE IF EXISTS users")
	if err != nil {
		return err
	}
	
	log.Println("Users table dropped successfully")
	return nil
}
