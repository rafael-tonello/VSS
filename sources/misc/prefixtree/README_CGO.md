# PrefixTree CGO Implementation

## Overview

This implementation embeds the C++ prefix tree code directly into the Go binary using CGO, eliminating the need for separate .so files.

## Files

### New Files Created

- **prefixtree.go** - Main Go interface with embedded C++ via CGO
- **prefixtree_cgo.cpp** - C++ wrapper functions that bridge Go and C++ code
- **combined_sources.cpp** - Includes all necessary C++ source files for compilation
- **prefixtree_test.go** - Test suite for the implementation

### Backup Files

- **pkv.go.old** - Original implementation using .so files (kept as backup)
- **old_main.cpp.bak** - Reference implementation for .so compilation (kept as reference)

## Key Features

- **No external dependencies**: All C++ code is compiled directly into the Go binary
- **Type-safe generics**: Uses Go generics for type-safe key-value operations
- **Full functionality**: Supports all operations from the original implementation:
  - Set/Get/Delete operations
  - Exists checks
  - SearchChilds with optional filtering
  - JSON serialization for complex types

## Usage Example

```go
package main

import (
    "fmt"
    "rtonello/vss/sources/misc/prefixtree"
)

func main() {
    // Create a new prefix tree for string values
    pkv, err := prefixtree.NewPkv[string]("data.db")
    if err != nil {
        panic(err)
    }
    defer pkv.Close()
    
    // Set a value
    pkv.Set("app/config/db/host", "localhost")
    
    // Get a value
    value, err := pkv.Get("app/config/db/host")
    if err != nil {
        panic(err)
    }
    fmt.Println(value) // Output: localhost
    
    // Check if key exists
    if pkv.Exists("app/config/db/host") {
        fmt.Println("Key exists!")
    }
    
    // Search for child keys
    children := pkv.SearchChilds("app/config", 10)
    for _, child := range children {
        fmt.Println(child)
    }
}
```

## Using with Structs

```go
type Config struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}

pkv, _ := prefixtree.NewPkv[Config]("config.db")
defer pkv.Close()

pkv.Set("server", Config{Host: "localhost", Port: 8080})

config, _ := pkv.Get("server")
fmt.Printf("Server: %s:%d\n", config.Host, config.Port)
```

## Building

The package builds automatically with:

```bash
go build
```

CGO will automatically compile the embedded C++ code during the build process.

## Testing

Run the test suite with:

```bash
go test -v
```

## Technical Details

### CGO Integration

The implementation uses CGO directives to:
- Set C++ compilation flags: `-std=c++17`
- Include necessary header paths
- Link against the C++ standard library

### Memory Management

- C strings are properly managed with `C.CString()` and `C.free()`
- Search results are automatically freed after conversion to Go slices
- The `Close()` method should be called to free C++ resources

### Thread Safety

The underlying C++ prefix tree implementation uses mutexes for thread-safe operations.

## Comparison with Previous Implementation

| Feature | Old (.so) | New (CGO) |
|---------|-----------|-----------|
| External files | Yes (.so file required) | No (all embedded) |
| Deployment | Must distribute .so | Single binary |
| Compilation | Separate make + go build | Single go build |
| Type safety | Runtime | Compile-time (generics) |
| Functionality | Full | Full |

## Notes

- The C++ code generates some harmless compiler warnings which can be ignored
- The implementation is fully compatible with the previous .so-based version
- All tests pass successfully
