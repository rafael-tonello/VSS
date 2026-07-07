package storage

import (
	"rtonello/vss/sources/misc"
)

// IStorage mirrors the C++ StorageInterface.h API in a Go-idiomatic way.
// Contract (inputs/outputs):
//   - Set(name, v): store value `v` at key `name`.
//   - Get(name, defaultValue): return stored value or `defaultValue` if missing.
//   - GetChilds(parentName): return only immediate child keys (full key names).
//   - HasValue(name): true if an exact value exists for `name`.
//   - DeleteValue(name, deleteChildsInACascade): remove key; if cascade, remove child keys too.
//   - ForEachChilds(parentName, f): iterate over immediate childs and call f(name, value).
//   - ForEachChildsParallel(parentName, f, concurrency): iterate immediate childs and call f concurrently
//     using up to `concurrency` goroutines; returns a channel that's closed when processing finishes.
//
// Error modes: methods are designed to be simple and not return errors for the common operations
// (to match the original interface). Implementations may panic only for programmer errors. If
// you need error reporting, prefer adding variants that return (T, error) on that implementation.
//
// Edge cases to consider in implementations:
// - Empty or root `parentName` value semantics ("" should be supported where appropriate).
// - Large number of children: ForEachChildsParallel accepts concurrency to control goroutine count.
// - Concurrent access: implementations must handle synchronization if used from multiple goroutines.
type IStorage interface {
	Set(name string, v misc.DynamicVar)
	Get(name string, defaultValue misc.DynamicVar) misc.DynamicVar

	// GetChilds returns only immediate childs (not grandchildren). Each returned string
	// should be the imediate key name for the child (not full key name).
	GetChildNames(parentName string) []string

	HasValue(name string) bool

	// DeleteValue removes the named key. If deleteChildsInACascade is true, remove child keys too.
	DeleteValue(name string, deleteChildsInACascade bool)

	// ForEachChilds iterates immediate child keys and calls f(childFullName, value).
	//ForEachChilds(parentName string, f func(string, misc.DynamicVar))

	// ForEachChildsParallel iterates immediate child keys and calls f concurrently.
	// The concurrency parameter limits the number of worker goroutines (must be >= 1).
	// It returns a channel that's closed when all executions are finished. Caller can optionally
	// wait for completion by reading from the returned channel (e.g. <-done).
	//ForEachChildsParallel(parentName string, f func(string, misc.DynamicVar), concurrency int) <-chan struct{}

	Finalize()
}
