package main

import (
	"database/sql"
	"log"
	"time"

	"github.com/faroedev/faroe"
	_ "github.com/mattn/go-sqlite3"
)

type mainStorageStruct struct {
	db         *sql.DB
	getStmt    *sql.Stmt
	setStmt    *sql.Stmt
	updateStmt *sql.Stmt
	deleteStmt *sql.Stmt
}

func (mainStorage *mainStorageStruct) Close() {
	// Close prepared statements first
	if mainStorage.getStmt != nil {
		mainStorage.getStmt.Close()
	}
	if mainStorage.setStmt != nil {
		mainStorage.setStmt.Close()
	}
	if mainStorage.updateStmt != nil {
		mainStorage.updateStmt.Close()
	}
	if mainStorage.deleteStmt != nil {
		mainStorage.deleteStmt.Close()
	}

	// Close database connection
	if mainStorage.db != nil {
		err := mainStorage.db.Close()
		if err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

func newMainStorage(fileName string) *mainStorageStruct {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Fatal(err)
	}

	// WAL mode is faster and more modern (https://sqlite.org/wal.html)
	// temp_store MEMORY will put more in memory vs in files
	// synchronous NORMAL still has full integrity when using WAL and is recommended in that case
	// 64MB cache size vs 8MB default
	setOptionsStmt := `
		PRAGMA journal_mode = WAL;
		PRAGMA temp_store = MEMORY;
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = -64000;
	`
	_, err = db.Exec(setOptionsStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, setOptionsStmt)
	}

	createTableStmt := `
		CREATE TABLE IF NOT EXISTS key_value (
			key TEXT PRIMARY KEY,
			counter INTEGER NOT NULL,
			expiration TEXT NOT NULL,
			value BLOB NOT NULL
		) STRICT;
	`
	_, err = db.Exec(createTableStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, createTableStmt)
	}

	// Prepare all statements
	getStmt, err := db.Prepare("SELECT value, counter FROM key_value WHERE key = ?")
	if err != nil {
		log.Fatalf("Failed to prepare get statement: %v", err)
	}

	setStmt, err := db.Prepare(`
		INSERT OR REPLACE INTO key_value (key, value, counter, expiration)
		VALUES (?, ?, 0, ?)
	`)
	if err != nil {
		getStmt.Close()
		log.Fatalf("Failed to prepare set statement: %v", err)
	}

	updateStmt, err := db.Prepare(`
		UPDATE key_value
		SET value = ?, counter = counter + 1, expiration = ?
		WHERE key = ? AND counter = ?
	`)
	if err != nil {
		getStmt.Close()
		setStmt.Close()
		log.Fatalf("Failed to prepare update statement: %v", err)
	}

	deleteStmt, err := db.Prepare("DELETE FROM key_value WHERE key = ?")
	if err != nil {
		getStmt.Close()
		setStmt.Close()
		updateStmt.Close()
		log.Fatalf("Failed to prepare delete statement: %v", err)
	}

	storage := &mainStorageStruct{
		db:         db,
		getStmt:    getStmt,
		setStmt:    setStmt,
		updateStmt: updateStmt,
		deleteStmt: deleteStmt,
	}
	return storage
}

func (mainStorage *mainStorageStruct) Get(key string) ([]byte, int32, error) {
	var value []byte
	var counter int32

	err := mainStorage.getStmt.QueryRow(key).Scan(&value, &counter)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, faroe.ErrMainStorageEntryNotFound
		}
		return nil, 0, err
	}

	return value, counter, nil
}

func (mainStorage *mainStorageStruct) Set(key string, value []byte, expiresAt time.Time) error {
	expirationStr := expiresAt.Format(time.RFC3339)

	_, err := mainStorage.setStmt.Exec(key, value, expirationStr)
	return err
}

func (mainStorage *mainStorageStruct) Update(key string, value []byte, expiresAt time.Time, counter int32) error {
	expirationStr := expiresAt.Format(time.RFC3339)

	result, err := mainStorage.updateStmt.Exec(value, expirationStr, key, counter)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return faroe.ErrMainStorageEntryNotFound
	}

	return nil
}

func (mainStorage *mainStorageStruct) Delete(key string) error {
	result, err := mainStorage.deleteStmt.Exec(key)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return faroe.ErrMainStorageEntryNotFound
	}

	return nil
}
