package prefixtree

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR} -I${SRCDIR}/common -I${SRCDIR}/storages
#cgo LDFLAGS: -lstdc++
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef int (*FilterFunction)(char* key, char* value);

void* pkv_new(char* filename, unsigned int tree_block_size);
void pkv_free(void* p);
void pkv_set(void* p, char* key, char* value);
char* pkv_get(void* p, char* key);
void pkv_delete(void* p, char* key);
int exists(void* p, char* key);
char** searchChilds(void* p, char* key, unsigned int maxResults, FilterFunction filter, int processChildsOfNodesExcludedByFilter);
void freeSearchChildsResult(void *p);

#ifdef __cplusplus
}
#endif
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

type Pkv[T any] struct {
	instance unsafe.Pointer
}

type PkvConfig struct {
	Filename      string
	TreeBlockSize uint
}

// Create a new prefix tree instance. The filename parameter is used to specify the file where the prefix tree will be stored. The tree_block_size parameter is used to specify the block size for the prefix tree (default is 64).
func NewPkv[T any](filename string, options ...func(*PkvConfig)) (*Pkv[T], error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	config := &PkvConfig{
		Filename:      filename,
		TreeBlockSize: 64,
	}

	for _, option := range options {
		option(config)
	}

	instance := C.pkv_new(cFilename, C.uint(config.TreeBlockSize))
	if instance == nil {
		return nil, errors.New("Failed to initialize prefix tree")
	}

	return &Pkv[T]{instance: instance}, nil
}

// optionals
// WithBlockSize allows setting a custom block size for the prefix tree. The block size determines how many key-value pairs are stored in each node of the tree. A larger block size can improve performance for large datasets, but may increase memory usage.
func WithBlockSize(blockSize uint) func(*PkvConfig) {
	return func(config *PkvConfig) {
		config.TreeBlockSize = blockSize
	}
}

// Close gracefully closes the prefix tree and releases all allocated resources.
// This method should be called when you're done using the prefix tree to prevent memory leaks.
// It's recommended to use defer immediately after creating the tree: defer pkv.Close()
func (p *Pkv[T]) Close() {
	if p.instance != nil {
		C.pkv_free(p.instance)
		p.instance = nil
	}
}

// Set stores a key-value pair in the prefix tree. If the key already exists, its value will be updated.
// The value is automatically serialized to JSON if it's not a string type.
// Keys typically use a hierarchical structure with separators (e.g., "app/config/database/host").
// Example:
//
//	pkv.Set("user/123/name", "John Doe")
//	pkv.Set("user/123/email", "john@example.com")
func (p *Pkv[T]) Set(key string, value T) {
	valueStr := anyToString(value, false)

	cKey := C.CString(key)
	cValue := C.CString(valueStr)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cValue))

	C.pkv_set(p.instance, cKey, cValue)
}

// Get retrieves the value associated with the given key from the prefix tree.
// Returns an error if the key does not exist or if the value is empty.
// The value is automatically deserialized from JSON if T is not a string type.
// Example:
//
//	name, err := pkv.Get("user/123/name")
//	if err != nil {
//	    log.Println("Key not found")
//	}
func (p *Pkv[T]) Get(key string) (T, error) {
	var ret T

	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	cValue := C.pkv_get(p.instance, cKey)
	if cValue == nil {
		return ret, errors.New("Key not found")
	}

	valueStr := C.GoString(cValue)
	if valueStr == "" {
		return ret, errors.New("Key not found")
	}

	ret = stringToAny[T](valueStr)
	return ret, nil
}

// GetOrDefault retrieves the value for the given key, or returns the default value if the key doesn't exist.
// This is a convenience method that avoids the need to check for errors when you have a sensible default.
// Example:
//
//	host := pkv.GetOrDefault("app/config/host", "localhost")
//	port := pkv.GetOrDefault("app/config/port", 8080)
func (p *Pkv[T]) GetOrDefault(key string, defaultValue T) T {
	ret, err := p.Get(key)
	if err != nil {
		return defaultValue
	}
	return ret
}

// Delete removes a key from the prefix tree by setting its value to empty.
// Note: The node structure itself is not removed (as it may have children), but the value is cleared.
// This should release storage space according to the underlying storage implementation.
// Example:
//
//	pkv.Delete("user/123/temp_token")
func (p *Pkv[T]) Delete(key string) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	C.pkv_delete(p.instance, cKey)
}

// Exists checks whether a key exists in the prefix tree and has a non-empty value.
// Returns true if the key exists with data, false otherwise.
// Example:
//
//	if pkv.Exists("user/123/name") {
//	    fmt.Println("User name is set")
//	}
func (p *Pkv[T]) Exists(key string) bool {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	result := C.exists(p.instance, cKey)
	return result != 0
}

// SearchChilds searches for all descendant keys that are children of the given prefix key.
// The search is recursive and includes children of children (all descendants).
// Only returns keys that have non-empty values.
//
// Parameters:
//   - key: The prefix key to search under (e.g., "app/config")
//   - maxResults: Maximum number of results to return (0 means return all)
//
// Returns a slice of full key paths for all matching descendants.
// Example:
//
//	// If tree contains: "app/config/db/host", "app/config/db/port", "app/config/api/key"
//	children := pkv.SearchChilds("app/config/db", 0)
//	// Returns: ["app/config/db/host", "app/config/db/port"]
func (p *Pkv[T]) SearchChilds(key string, maxResults uint) []string {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	result := C.searchChilds(p.instance, cKey, C.uint(maxResults), nil, 1)
	if result == nil {
		return []string{}
	}
	defer C.freeSearchChildsResult(p.instance)

	// Convert C char** to Go slice
	var ret []string
	ptr := uintptr(unsafe.Pointer(result))
	for {
		charPtr := *(**C.char)(unsafe.Pointer(ptr))
		if charPtr == nil {
			break
		}
		ret = append(ret, C.GoString(charPtr))
		ptr += unsafe.Sizeof(uintptr(0))
	}

	return ret
}

// SearchChildsFiltering searches for descendant keys with a custom filter function.
// Similar to SearchChilds, but allows you to filter results based on both key and value.
// The filter function is called for each descendant, and only those returning true are included.
//
// Parameters:
//   - key: The prefix key to search under
//   - maxResults: Maximum number of results to return (0 means return all)
//   - filter: Function that receives (key, value) and returns true to include the result
//
// Example:
//
//	// Find all users with age over 18
//	adults := pkv.SearchChildsFiltering("users", 0, func(key string, user User) bool {
//	    return user.Age > 18
//	})
func (p *Pkv[T]) SearchChildsFiltering(key string, maxResults uint, filter func(string, T) bool) []string {
	// TODO: Implement filter callback from Go to C
	// For now, get all results and filter in Go
	allResults := p.SearchChilds(key, maxResults)

	var filtered []string
	for _, resultKey := range allResults {
		value, err := p.Get(resultKey)
		if err == nil && filter(resultKey, value) {
			filtered = append(filtered, resultKey)
			if maxResults > 0 && uint(len(filtered)) >= maxResults {
				break
			}
		}
	}

	return filtered
}

func anyToString(value interface{}, enableJsonOutputIdent bool) string {
	msgsType := reflect.TypeOf(value)

	valueStr := ""

	switch msgsType.Kind() {
	case reflect.String:
		valueStr = value.(string)
	default:
		var valueByte []byte
		var err error
		if enableJsonOutputIdent {
			valueByte, err = json.MarshalIndent(&value, "", "  ")
		} else {
			valueByte, err = json.Marshal(&value)
		}

		if err == nil {
			valueStr = string(valueByte)
		} else {
			valueStr = fmt.Sprintf("%v", value)
		}
	}

	return valueStr
}

func stringToAny[T any](valueStr string) T {
	var value T
	err := json.Unmarshal([]byte(valueStr), &value)
	if err != nil {
		// If unmarshal fails and T is string, return the string directly
		var zeroVal T
		if reflect.TypeOf(zeroVal).Kind() == reflect.String {
			return any(valueStr).(T)
		}
	}
	return value
}
