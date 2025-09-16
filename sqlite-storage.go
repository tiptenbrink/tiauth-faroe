package main

import (
	"database/sql"
	"log"
	"time"

	"github.com/faroedev/faroe"
	"github.com/mattn/go-sqlite3"
)

type storageStruct struct {
	db         *sql.DB
	getStmt    *sql.Stmt
	addStmt    *sql.Stmt
	updateStmt *sql.Stmt
	deleteStmt *sql.Stmt
}

func (storage *storageStruct) Close() {
	// Close prepared statements first
	if storage.getStmt != nil {
		storage.getStmt.Close()
	}
	if storage.addStmt != nil {
		storage.addStmt.Close()
	}
	if storage.updateStmt != nil {
		storage.updateStmt.Close()
	}
	if storage.deleteStmt != nil {
		storage.deleteStmt.Close()
	}

	// Close database connection
	if storage.db != nil {
		err := storage.db.Close()
		if err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

func newStorage(fileName string) *storageStruct {
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

	addStmt, err := db.Prepare(`
		INSERT INTO key_value (key, value, counter, expiration) VALUES (?, ?, 0, ?)
	`)
	if err != nil {
		getStmt.Close()
		log.Fatalf("Failed to prepare add statement: %v", err)
	}

	updateStmt, err := db.Prepare(`
		UPDATE key_value
		SET value = ?, counter = counter + 1, expiration = ?
		WHERE key = ? AND counter = ?
	`)
	if err != nil {
		getStmt.Close()
		addStmt.Close()
		log.Fatalf("Failed to prepare update statement: %v", err)
	}

	deleteStmt, err := db.Prepare("DELETE FROM key_value WHERE key = ?")
	if err != nil {
		getStmt.Close()
		addStmt.Close()
		updateStmt.Close()
		log.Fatalf("Failed to prepare delete statement: %v", err)
	}

	storage := &storageStruct{
		db:         db,
		getStmt:    getStmt,
		addStmt:    addStmt,
		updateStmt: updateStmt,
		deleteStmt: deleteStmt,
	}
	return storage
}

func (storage *storageStruct) Get(key string) ([]byte, int32, error) {
	var value []byte
	var counter int32

	err := storage.getStmt.QueryRow(key).Scan(&value, &counter)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, faroe.ErrStorageEntryNotFound
		}
		return nil, 0, err
	}

	return value, counter, nil
}

func (storage *storageStruct) Add(key string, value []byte, expiresAt time.Time) error {
	expirationStr := expiresAt.Format(time.RFC3339)
	_, err := storage.addStmt.Exec(key, value, expirationStr)
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return faroe.ErrStorageEntryAlreadyExists
		}
	}
	return err
}

func (storage *storageStruct) Update(key string, value []byte, expiresAt time.Time, counter int32) error {
	expirationStr := expiresAt.Format(time.RFC3339)

	result, err := storage.updateStmt.Exec(value, expirationStr, key, counter)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return faroe.ErrStorageEntryNotFound
	}

	return nil
}

func (storage *storageStruct) Delete(key string) error {
	result, err := storage.deleteStmt.Exec(key)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return faroe.ErrStorageEntryNotFound
	}

	return nil
}

func (storage *storageStruct) Clear() error {
	_, err := storage.db.Exec("DELETE FROM key_value")
	return err
}
