package db

import (
    "log/slog"
    "time"

    "go.etcd.io/bbolt"
)

var (
    DB *bbolt.DB
)

// Init initializes the BoltDB database
func Init(dbPath string) error {
    var err error
    DB, err = bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 5 * time.Second})
    if err != nil {
        return err
    }

    // Create all buckets if they don't exist
    return DB.Update(func(tx *bbolt.Tx) error {
        buckets := []string{
            BucketConfig,
            BucketRepos,
            BucketPRs,
            BucketLogs,
            BucketQueueItems,
            BucketUsers,
            BucketSyncHistory,
        }
        for _, bucket := range buckets {
            if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
                return err
            }
        }
        return nil
    })
}

// Close closes the database
func Close() error {
    if DB != nil {
        return DB.Close()
    }
    return nil
}

// Update wraps bbolt Update
func Update(fn func(tx *bbolt.Tx) error) error {
    return DB.Update(fn)
}

// View wraps bbolt View
func View(fn func(tx *bbolt.Tx) error) error {
    return DB.View(fn)
}

// Put stores a key-value pair in the specified bucket
func Put(bucket, key string, value []byte) error {
    return DB.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(bucket))
        if b == nil {
            return bbolt.ErrBucketNotFound
        }
        return b.Put([]byte(key), value)
    })
}

// Get retrieves a value by key from the specified bucket
func Get(bucket, key string) ([]byte, error) {
    var result []byte
    err := DB.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(bucket))
        if b == nil {
            return bbolt.ErrBucketNotFound
        }
        val := b.Get([]byte(key))
        if val != nil {
            result = make([]byte, len(val))
            copy(result, val)
        }
        return nil
    })
    return result, err
}

// Delete removes a key from the specified bucket
func Delete(bucket, key string) error {
    return DB.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(bucket))
        if b == nil {
            return bbolt.ErrBucketNotFound
        }
        return b.Delete([]byte(key))
    })
}

// ForEach iterates over all key-value pairs in the specified bucket
func ForEach(bucket string, fn func(key, value []byte) error) error {
    return DB.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(bucket))
        if b == nil {
            return bbolt.ErrBucketNotFound
        }
        return b.ForEach(func(k, v []byte) error {
            return fn(k, v)
        })
    })
}

// RunMigrations runs database migrations
func RunMigrations() error {
    slog.Info("running database migrations")
    return nil
}
