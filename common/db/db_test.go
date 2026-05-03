package db

import (
	"testing"

	"go.etcd.io/bbolt"
)

func initTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	err := Init(dir + "/test.db")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { Close() })
}

func TestInitAndClose(t *testing.T) {
	initTestDB(t)

	if DB == nil {
		t.Fatal("DB should be initialized")
	}
}

func TestBucketsCreated(t *testing.T) {
	initTestDB(t)

	// Verify bucket is created
	err := DB.View(func(tx *bbolt.Tx) error {
		buckets := []string{
			BucketConfig,
			BucketRepos,
			BucketPRs,
			BucketLogs,
			BucketQueueItems,
			BucketUsers,
			BucketSyncHistory,
		}
		for _, bucketName := range buckets {
			b := tx.Bucket([]byte(bucketName))
			if b == nil {
				t.Errorf("bucket %q not found", bucketName)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
}

func TestPutAndGet(t *testing.T) {
	initTestDB(t)

	// Test Put and Get
	err := Put("prs", "pr-123", []byte(`{"id":"pr-123","state":"open"}`))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := Get("prs", "pr-123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(val) != `{"id":"pr-123","state":"open"}` {
		t.Errorf("Get() = %q, want %q", string(val), `{"id":"pr-123","state":"open"}`)
	}
}

func TestGet_NonExistent(t *testing.T) {
	initTestDB(t)

	val, err := Get("prs", "non-existent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if val != nil {
		t.Errorf("Get() for non-existent key should return nil, got %q", string(val))
	}
}

func TestDelete(t *testing.T) {
	initTestDB(t)

	// Insert first
	err := Put("prs", "pr-123", []byte("test-data"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify exists
	val, _ := Get("prs", "pr-123")
	if val == nil {
		t.Fatal("key should exist before delete")
	}

	// Delete
	err = Delete("prs", "pr-123")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	val, _ = Get("prs", "pr-123")
	if val != nil {
		t.Errorf("key should not exist after delete, got %q", string(val))
	}
}

func TestDelete_NonExistent(t *testing.T) {
	initTestDB(t)

	err := Delete("prs", "non-existent")
	if err != nil {
		t.Fatalf("Delete non-existent key should not error: %v", err)
	}
}

func TestForEach(t *testing.T) {
	initTestDB(t)

	// Insert multiple data
	testData := map[string]string{
		"pr-1": "data-1",
		"pr-2": "data-2",
		"pr-3": "data-3",
	}

	for k, v := range testData {
		err := Put("prs", k, []byte(v))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Use ForEach to iterate
	count := 0
	found := make(map[string]bool)
	err := ForEach("prs", func(key, value []byte) error {
		count++
		found[string(key)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}

	if count != len(testData) {
		t.Errorf("ForEach visited %d items, want %d", count, len(testData))
	}

	for k := range testData {
		if !found[k] {
			t.Errorf("key %q not visited by ForEach", k)
		}
	}
}

func TestForEach_EmptyBucket(t *testing.T) {
	initTestDB(t)

	count := 0
	err := ForEach("prs", func(key, value []byte) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}

	if count != 0 {
		t.Errorf("ForEach on empty bucket should visit 0 items, got %d", count)
	}
}

func TestUpdate(t *testing.T) {
	initTestDB(t)

	err := Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("prs"))
		if b == nil {
			return bbolt.ErrBucketNotFound
		}
		return b.Put([]byte("test-key"), []byte("test-value"))
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify write succeeded
	val, _ := Get("prs", "test-key")
	if string(val) != "test-value" {
		t.Errorf("Get() = %q, want test-value", string(val))
	}
}

func TestView(t *testing.T) {
	initTestDB(t)

	// Write data first
	err := Put("prs", "view-test", []byte("view-value"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("prs"))
		if b == nil {
			return bbolt.ErrBucketNotFound
		}
		val := b.Get([]byte("view-test"))
		if string(val) != "view-value" {
			t.Errorf("in View: val = %q, want view-value", string(val))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
}

func TestPut_InvalidBucket(t *testing.T) {
	initTestDB(t)

	err := Put("invalid-bucket", "key", []byte("value"))
	if err == nil {
		t.Error("Put to invalid bucket should return error")
	}
}

func TestGet_InvalidBucket(t *testing.T) {
	initTestDB(t)

	_, err := Get("invalid-bucket", "key")
	if err == nil {
		t.Error("Get from invalid bucket should return error")
	}
}

func TestDelete_InvalidBucket(t *testing.T) {
	initTestDB(t)

	err := Delete("invalid-bucket", "key")
	if err == nil {
		t.Error("Delete from invalid bucket should return error")
	}
}

func TestForEach_InvalidBucket(t *testing.T) {
	initTestDB(t)

	err := ForEach("invalid-bucket", func(key, value []byte) error {
		return nil
	})
	if err == nil {
		t.Error("ForEach on invalid bucket should return error")
	}
}
