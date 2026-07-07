package prefixtree

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPkvBasicOperations(t *testing.T) {
	// Clean up test file
	testFile := "/tmp/test_prefixtree.db"
	defer os.Remove(testFile)

	// Create a new prefix tree
	pkv, err := NewPkv[string](testFile)
	if err != nil {
		t.Fatalf("Failed to create Pkv: %v", err)
	}
	defer pkv.Close()

	// Test Set and Get
	key := "test/key/1"
	value := "hello world"

	pkv.Set(key, value)

	result, err := pkv.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if result != value {
		t.Errorf("Expected %s, got %s", value, result)
	}

	// Test GetOrDefault
	result2 := pkv.GetOrDefault("nonexistent", "default value")
	if result2 != "default value" {
		t.Errorf("Expected 'default value', got %s", result2)
	}

	// Test Exists
	if !pkv.Exists(key) {
		t.Error("Key should exist")
	}

	if pkv.Exists("nonexistent") {
		t.Error("Nonexistent key should not exist")
	}

	// Test Delete
	pkv.Delete(key)
	if pkv.Exists(key) {
		t.Error("Key should not exist after deletion")
	}
}

func TestPkvSearchChilds(t *testing.T) {
	// Clean up test file
	testFile := "/tmp/test_prefixtree_search.db"
	defer os.Remove(testFile)

	// Create a new prefix tree
	pkv, err := NewPkv[string](testFile)
	if err != nil {
		t.Fatalf("Failed to create Pkv: %v", err)
	}
	defer pkv.Close()

	// Add some test data
	pkv.Set("app/config/db/host", "localhost")
	pkv.Set("app/config/db/port", "5432")
	pkv.Set("app/config/api/key", "secret")
	pkv.Set("app/config/api/url", "https://api.example.com")

	// Search for children with prefix
	results := pkv.SearchChilds("app/config/db", 10)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify results contain expected keys
	expectedKeys := map[string]bool{
		"app/config/db/host": true,
		"app/config/db/port": true,
	}

	for _, key := range results {
		if !expectedKeys[key] {
			t.Errorf("Unexpected key in results: %s", key)
		}
	}
}

func TestPkvWithStructs(t *testing.T) {
	// Clean up test file
	testFile := "/tmp/test_prefixtree_struct.db"
	defer os.Remove(testFile)

	type Config struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	// Create a new prefix tree for Config structs
	pkv, err := NewPkv[Config](testFile)
	if err != nil {
		t.Fatalf("Failed to create Pkv: %v", err)
	}
	defer pkv.Close()

	// Test Set and Get with struct
	key := "server/config"
	value := Config{Host: "localhost", Port: 8080}

	pkv.Set(key, value)

	result, err := pkv.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if result.Host != value.Host || result.Port != value.Port {
		t.Errorf("Expected %+v, got %+v", value, result)
	}
}

func TestPkvPerformance(t *testing.T) {
	// Clean up test file
	testFile := "/tmp/test_prefixtree_performance.db"
	defer os.Remove(testFile)

	// Create a new prefix tree
	pkv, err := NewPkv[string](testFile)
	if err != nil {
		t.Fatalf("Failed to create Pkv: %v", err)
	}
	defer pkv.Close()

	numRecords := 1000

	// Test Write Performance
	fmt.Printf("\n=== Performance Test with %d records ===\n", numRecords)

	writeStart := time.Now()
	for i := 0; i < numRecords; i++ {
		key := fmt.Sprintf("benchmark/key/%d", i)
		value := fmt.Sprintf("value_%d_with_some_data_to_make_it_realistic", i)
		pkv.Set(key, value)
	}
	writeDuration := time.Since(writeStart)
	avgWriteTime := writeDuration / time.Duration(numRecords)

	fmt.Printf("Write Performance:\n")
	fmt.Printf("  Total time: %v\n", writeDuration)
	fmt.Printf("  Average time per write: %v\n", avgWriteTime)
	fmt.Printf("  Writes per second: %.2f\n", float64(numRecords)/writeDuration.Seconds())

	// Test Read Performance
	readStart := time.Now()
	for i := 0; i < numRecords; i++ {
		key := fmt.Sprintf("benchmark/key/%d", i)
		_, err := pkv.Get(key)
		if err != nil {
			t.Errorf("Failed to read key %s: %v", key, err)
		}
	}
	readDuration := time.Since(readStart)
	avgReadTime := readDuration / time.Duration(numRecords)

	fmt.Printf("\nRead Performance:\n")
	fmt.Printf("  Total time: %v\n", readDuration)
	fmt.Printf("  Average time per read: %v\n", avgReadTime)
	fmt.Printf("  Reads per second: %.2f\n", float64(numRecords)/readDuration.Seconds())

	// Test Exists Performance
	existsStart := time.Now()
	for i := 0; i < numRecords; i++ {
		key := fmt.Sprintf("benchmark/key/%d", i)
		if !pkv.Exists(key) {
			t.Errorf("Key %s should exist", key)
		}
	}
	existsDuration := time.Since(existsStart)
	avgExistsTime := existsDuration / time.Duration(numRecords)

	fmt.Printf("\nExists Check Performance:\n")
	fmt.Printf("  Total time: %v\n", existsDuration)
	fmt.Printf("  Average time per check: %v\n", avgExistsTime)
	fmt.Printf("  Checks per second: %.2f\n", float64(numRecords)/existsDuration.Seconds())

	// Test SearchChilds Performance
	searchStart := time.Now()
	results := pkv.SearchChilds("benchmark/key", 0)
	searchDuration := time.Since(searchStart)

	fmt.Printf("\nSearchChilds Performance:\n")
	fmt.Printf("  Total time: %v\n", searchDuration)
	fmt.Printf("  Found %d results\n", len(results))

	if len(results) != numRecords {
		t.Errorf("Expected %d search results, got %d", numRecords, len(results))
	}

	// Test Delete Performance
	deleteStart := time.Now()
	for i := 0; i < numRecords; i++ {
		key := fmt.Sprintf("benchmark/key/%d", i)
		pkv.Delete(key)
	}
	deleteDuration := time.Since(deleteStart)
	avgDeleteTime := deleteDuration / time.Duration(numRecords)

	fmt.Printf("\nDelete Performance:\n")
	fmt.Printf("  Total time: %v\n", deleteDuration)
	fmt.Printf("  Average time per delete: %v\n", avgDeleteTime)
	fmt.Printf("  Deletes per second: %.2f\n", float64(numRecords)/deleteDuration.Seconds())

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total operations: %d writes + %d reads + %d exists + %d deletes = %d\n",
		numRecords, numRecords, numRecords, numRecords, numRecords*4)
	totalTime := writeDuration + readDuration + existsDuration + deleteDuration
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Overall operations per second: %.2f\n\n", float64(numRecords*4)/totalTime.Seconds())
}
