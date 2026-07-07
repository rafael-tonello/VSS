package storage

// aversion of rancachedb that uses prefix tree instead a text file to store/load data

import (
	"fmt"
	"os"
	"path/filepath"

	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/misc/logger"
	"rtonello/vss/sources/misc/prefixtree"
)

type RamCachedDBPkv struct {
	RamCachedDB
	db *prefixtree.Pkv[string]
}

// NewRamCacheDBWithOptions creates and returns an initialized RamCacheDB with options.
// If dataDir is empty a default is used; if dumpIntervalMs <= 0 a default 5min is used.
func NewRamCachedDBPkv(logger logger.ILogger, dataDir string, dumpIntervalMs int) (IStorage, error) {
	r := &RamCachedDBPkv{}
	r.root.childs = make(map[string]*ramNode)
	r.root.imediateName = ""
	r.log = logger.GetNamedLogger("Storage::RamCacheDB")

	if dataDir == "" {
		r.dataDir = "./data/database"
	} else {
		r.dataDir = dataDir
	}

	//check if dtadir exists
	if _, err := os.Stat(r.dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(r.dataDir, 0o755); err != nil {
			r.log.Error("Error creating data directory: " + err.Error())
			return nil, fmt.Errorf("error creating data directory: %w", err)
		}
	}

	tmp, err := prefixtree.NewPkv[string](filepath.Join(r.dataDir, "ramcachedbpkv.db"))
	if err != nil {
		r.log.Error("Error initializing prefix tree: " + err.Error())
		return nil, fmt.Errorf("error initializing prefix tree: %w", err)
	}

	r.db = tmp

	if dumpIntervalMs <= 0 {
		r.dumpIntervalMs = 5 * 60 * 1000 // 5 minutes
	} else {
		r.dumpIntervalMs = dumpIntervalMs
	}

	r.stopCh = make(chan struct{})
	// load existing data
	if err := r.loadPkv(); err != nil {
		r.log.Error("Error loading RamCacheDB from disk: " + err.Error())
		r.log.Warning("Continuing with empty RamCacheDB.")
	}

	//load data from disk
	if err := r.load(); err != nil {
		r.log.Error("Error loading RamCacheDB from disk: " + err.Error())
		r.log.Warning("Continuing with empty RamCacheDB.")
	}

	go r.backgroundDumper(r.dumpPkv)
	return r, nil
}

// redirects to NewRamCacheDB with auto defined parameters
func NewRamCachedDB_2Pkv(logger logger.ILogger) (IStorage, error) {
	return NewRamCachedDBPkv(logger, "", 0)
}

// dump expects r.mu to be locked. It writes the dump file to disk and returns any error.
func (r *RamCachedDBPkv) dumpPkv() {
	r.log.Trace("Dumping RamCacheDB to disk...")
	r.dumpNodesPkv(&r.root, "")
}

func (r *RamCachedDBPkv) dumpNodesPkv(current *ramNode, currentParentName string) {
	prefix := ""
	if currentParentName != "" {
		prefix = currentParentName + "."
	}
	for k, c := range current.childs {
		//check if c.tags["changed"] exists and is true, if not skip this node (and its childs)
		if changed, ok := c.tags["changed"].(bool); ok {
			if changed {
				name := prefix + k
				value := c.value.GetString()

				if value != "" {
					r.db.Set(name, value)
				} else {
					r.db.Delete(name)
				}
			}
		}
	}
}

// Load reads the dump file and populates the in-memory cache. Existing nodes are preserved/extended.
func (r *RamCachedDBPkv) loadPkv() error {
	maxuintvalue := ^uint(0)
	childNames := r.db.SearchChilds("", maxuintvalue) //force loading all keys in memory, so we can use GetOrDefault to load values

	for _, name := range childNames {
		value := r.db.GetOrDefault(name, "")
		if value != "" {
			r.Set(name, misc.NewDynamicVar(value))
		}
	}

	return nil
}
