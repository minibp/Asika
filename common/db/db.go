package db

import (
    "encoding/json"
    "fmt"
    "log/slog"
    "time"

    "asika/common/models"
    "go.etcd.io/bbolt"
)

var (
    DB *bbolt.DB
)

// Init initializes the BoltDB database
func Init(dbPath string) error {
    var err error
    DB, err = bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 30 * time.Second})
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
            BucketPRIndexByID,
            BucketPRIndexByRG,
            BucketWebhookRetries,
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

// PutPRWithIndex stores a PR and updates indices atomically
func PutPRWithIndex(key string, value []byte, prID, repoGroup string, prNumber int) error {
    return DB.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(BucketPRs))
        if b == nil {
            return bbolt.ErrBucketNotFound
        }
        if err := b.Put([]byte(key), value); err != nil {
            return err
        }

        if prID != "" {
            idxB := tx.Bucket([]byte(BucketPRIndexByID))
            if idxB != nil {
                idxB.Put([]byte(prID), []byte(key))
            }
        }

        if repoGroup != "" {
            idxB := tx.Bucket([]byte(BucketPRIndexByRG))
            if idxB != nil {
                rgKey := fmt.Sprintf("%s:%d", repoGroup, prNumber)
                idxB.Put([]byte(rgKey), []byte(key))
            }
        }

        return nil
    })
}

// GetPRByIndex tries to find a PR using indices first, falling back to scan
func GetPRByIndex(prID, repoGroup string, prNumber int) ([]byte, error) {
    var result []byte

    err := DB.View(func(tx *bbolt.Tx) error {
        // Try index by ID
        if prID != "" {
            idxB := tx.Bucket([]byte(BucketPRIndexByID))
            if idxB != nil {
                if key := idxB.Get([]byte(prID)); key != nil {
                    b := tx.Bucket([]byte(BucketPRs))
                    if b != nil {
                        if val := b.Get(key); val != nil {
                            result = make([]byte, len(val))
                            copy(result, val)
                            return nil
                        }
                    }
                }
            }
        }

        // Try index by repo_group + number
        if repoGroup != "" && prNumber > 0 {
            idxB := tx.Bucket([]byte(BucketPRIndexByRG))
            if idxB != nil {
                rgKey := fmt.Sprintf("%s:%d", repoGroup, prNumber)
                if key := idxB.Get([]byte(rgKey)); key != nil {
                    b := tx.Bucket([]byte(BucketPRs))
                    if b != nil {
                        if val := b.Get(key); val != nil {
                            result = make([]byte, len(val))
                            copy(result, val)
                            return nil
                        }
                    }
                }
            }
        }

        return nil
    })
    
    return result, err
}

// PutWebhookRetry stores a webhook retry entry
func PutWebhookRetry(retry *models.WebhookRetry) error {
    data, err := json.Marshal(retry)
    if err != nil {
        return err
    }
    return Put(BucketWebhookRetries, retry.ID, data)
}

// GetWebhookRetry retrieves a webhook retry by ID
func GetWebhookRetry(id string) (*models.WebhookRetry, error) {
    data, err := Get(BucketWebhookRetries, id)
    if err != nil || data == nil {
        return nil, err
    }
    var retry models.WebhookRetry
    if err := json.Unmarshal(data, &retry); err != nil {
        return nil, err
    }
    return &retry, nil
}

// DeleteWebhookRetry removes a webhook retry entry
func DeleteWebhookRetry(id string) error {
    return Delete(BucketWebhookRetries, id)
}

// ForEachWebhookRetry iterates over all webhook retry entries
func ForEachWebhookRetry(fn func(retry *models.WebhookRetry) error) error {
    return ForEach(BucketWebhookRetries, func(key, value []byte) error {
        var retry models.WebhookRetry
        if err := json.Unmarshal(value, &retry); err != nil {
            return nil // skip invalid entries
        }
        return fn(&retry)
    })
}

// GetDueWebhookRetries returns retries that are due for retry (NextRetry <= now)
func GetDueWebhookRetries(now time.Time) ([]*models.WebhookRetry, error) {
    var due []*models.WebhookRetry
    err := ForEachWebhookRetry(func(retry *models.WebhookRetry) error {
        if retry.NextRetry.IsZero() || retry.NextRetry.After(now) {
            return nil
        }
        due = append(due, retry)
        return nil
    })
    return due, err
}

// AppendAuditLog adds an audit log entry to the database
func AppendAuditLog(level, message string, ctx map[string]interface{}) error {
    log := models.AuditLog{
        Timestamp: time.Now(),
        Level:     level,
        Message:   message,
        Context:   ctx,
    }
    data, err := json.Marshal(log)
    if err != nil {
        return err
    }
    // Use timestamp + random as key to allow multiple entries with same timestamp
    key := fmt.Sprintf("%d_%s", log.Timestamp.UnixNano(), message[:min(len(message), 8)])
    return Put(BucketLogs, key, data)
}
