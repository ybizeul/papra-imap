package state

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

var processedBucket = []byte("processed")

type DB struct {
	db *bolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(processedBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) IsProcessed(messageID string) (bool, error) {
	var found bool
	err := d.db.View(func(tx *bolt.Tx) error {
		found = tx.Bucket(processedBucket).Get([]byte(messageID)) != nil
		return nil
	})
	return found, err
}

func (d *DB) MarkProcessed(messageID string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(processedBucket).Put(
			[]byte(messageID),
			[]byte(time.Now().UTC().Format(time.RFC3339)),
		)
	})
}

func (d *DB) Close() error {
	return d.db.Close()
}
