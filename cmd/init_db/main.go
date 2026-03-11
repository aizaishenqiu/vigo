package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	// 1. Create Database
	fmt.Println("Creating database jw_platform_db...")
	cmd := exec.Command("mysql", "-u", "root", "-pshine20", "-e", "CREATE DATABASE IF NOT EXISTS jw_platform_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to create database: %v\n", err)
		// Don't return, maybe it already exists and we just failed to create (permission?) but we can try importing.
	}

	// 2. Import SQL
	fmt.Println("Importing SQL file...")
	file, err := os.Open(`M:\www\foucui-edu\edu_docs\data\jw_platform_db.sql`)
	if err != nil {
		fmt.Printf("Failed to open SQL file: %v\n", err)
		return
	}
	defer file.Close()

	cmdImport := exec.Command("mysql", "-u", "root", "-pshine20", "jw_platform_db")
	cmdImport.Stdin = file
	cmdImport.Stdout = os.Stdout
	cmdImport.Stderr = os.Stderr
	if err := cmdImport.Run(); err != nil {
		fmt.Printf("Failed to import SQL: %v\n", err)
		return
	}
	fmt.Println("SQL imported successfully.")
}
