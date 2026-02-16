package storage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/misc/logger"
)

// ramNode represents a node in the in-memory tree storage.
type ramNode struct {
	tags         map[string]any
	imediateName string
	parent       *ramNode
	childs       map[string]*ramNode
	value        misc.DynamicVar
}

// RamCachedDB is a simple in-memory tree-based storage that mirrors the C++ RamCachedDB
// behavior: keys are dot-separated (e.g. "a.b.c"), GetChilds returns immediate child names
// (not full names) and ForEachChilds passes the full child key to the callback.
type RamCachedDB struct {
	mu             sync.RWMutex
	root           ramNode
	pendingChanges atomic.Bool

	// dump configuration
	dataDir        string
	dumpIntervalMs int
	stopCh         chan struct{}
	log            logger.INamedLogger
}

// NewRamCacheDBWithOptions creates and returns an initialized RamCacheDB with options.
// If dataDir is empty a default is used; if dumpIntervalMs <= 0 a default 5min is used.
func NewRamCacheDB(logger logger.ILogger, dataDir string, dumpIntervalMs int) IStorage {
	r := &RamCachedDB{}
	r.root.childs = make(map[string]*ramNode)
	r.root.imediateName = ""
	r.log = logger.GetNamedLogger("Storage::RamCacheDB")

	if dataDir == "" {
		r.dataDir = "./data/database"
	} else {
		r.dataDir = dataDir
	}
	if dumpIntervalMs <= 0 {
		r.dumpIntervalMs = 5 * 60 * 1000 // 5 minutes
	} else {
		r.dumpIntervalMs = dumpIntervalMs
	}

	r.stopCh = make(chan struct{})
	// load existing data
	if err := r.load(); err != nil {
		r.log.Error("Error loading RamCacheDB from disk: " + err.Error())
		r.log.Warning("Continuing with empty RamCacheDB.")
	}

	//load data from disk
	if err := r.load(); err != nil {
		r.log.Error("Error loading RamCacheDB from disk: " + err.Error())
		r.log.Warning("Continuing with empty RamCacheDB.")
	}

	go r.backgroundDumper(r.dump)
	return r
}

// redirects to NewRamCacheDB with auto defined parameters
func NewRamCacheDB_2(logger logger.ILogger) IStorage {
	return NewRamCacheDB(logger, "", 0)
}

// Close stops the background dumper and performs a final dump.
func (r *RamCachedDB) Finalize() {
	// close stopCh once (safe non-blocking check)
	select {
	case <-r.stopCh:
		// already closed
		return
	default:
		close(r.stopCh)
	}
}

func (r *RamCachedDB) backgroundDumper(dumpFunction func()) {
	ticker := time.NewTicker(time.Duration(r.dumpIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {

		select {
		case <-r.stopCh:
			r.mu.Lock()
			if r.pendingChanges.Load() {
				dumpFunction()
				r.pendingChanges.Store(false)
			}
			r.mu.Unlock()

			return
		case <-ticker.C:
			r.mu.Lock()
			if r.pendingChanges.Load() {
				dumpFunction()
				r.pendingChanges.Store(false)
			}
			r.mu.Unlock()
		}
	}
}

// helper: traverse tree, creating nodes if create=true
func (r *RamCachedDB) scrollTree(name string, curr *ramNode, readOnly bool) *ramNode {
	currName := name
	remainingName := ""
	if strings.Contains(name, ".") {
		indx := strings.Index(name, ".")
		currName = name[:indx]
		remainingName = name[indx+1:]
	}

	if curr.childs[currName] == nil {
		{
			if readOnly {
				return nil
			}

			curr.childs[currName] = &ramNode{
				imediateName: currName,
				parent:       curr,
				childs:       make(map[string]*ramNode),
				value:        misc.NewEmptyDynamicVar(),
			}

			//if curr.fullName != "" {
			//	curr.childs[currName].fullName = curr.fullName + "." + currName
			//} else {
			//	curr.childs[currName].fullName = currName
			//}

		}
	}

	if remainingName == "" {
		return curr.childs[currName]
	}

	return r.scrollTree(remainingName, curr.childs[currName], readOnly)
}

// Set stores value v at key name.
func (r *RamCachedDB) Set(name string, v misc.DynamicVar) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.scrollTree(name, &r.root, false)
	if item != nil {
		item.value = v
		r.pendingChanges.Store(true)
	}
}

// Get returns the stored value or defaultValue if not present.
func (r *RamCachedDB) Get(name string, defaultValue misc.DynamicVar) misc.DynamicVar {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item := r.scrollTree(name, &r.root, true)
	if item != nil {
		return item.value
	}
	return defaultValue
}

// GetChilds returns immediate child names (not full names) for parentName.
func (r *RamCachedDB) GetChildNames(parentName string) []string {
	ret := []string{}
	if parentName == "" {
		//get root childs
		r.mu.RLock()
		defer r.mu.RUnlock()
		for k := range r.root.childs {
			ret = append(ret, k)
		}
		return ret
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	item := r.scrollTree(parentName, &r.root, true)
	if item == nil {
		return []string{}
	}

	ret = make([]string, 0, len(item.childs))
	if item == nil || item.childs == nil {
		return []string{}
	}
	for k := range item.childs {
		ret = append(ret, k)
	}
	return ret
}

// HasValue returns true if the node for name exists (matches C++ semantics).
func (r *RamCachedDB) HasValue(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item := r.scrollTree(name, &r.root, true)
	return item != nil
}

// DeleteValue removes the value at name; if deleteChildsInACascade is true, remove children as well.
func (r *RamCachedDB) DeleteValue(name string, deleteChildsInACascade bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.scrollTree(name, &r.root, true)
	if item == nil {
		return
	}
	// clear value
	item.value = misc.NewEmptyDynamicVar()
	if deleteChildsInACascade {
		item.childs = make(map[string]*ramNode)
	}
	if len(item.childs) == 0 && item.parent != nil {
		delete(item.parent.childs, item.imediateName)
	}
	r.pendingChanges.Store(true)
}

// ForEachChilds iterates immediate childs and calls f(fullChildName, value).
// func (r *RamCachedDB) ForEachChilds(parentName string, f func(string, misc.DynamicVar)) {
// 	r.mu.RLock()
// 	defer r.mu.RUnlock()
// 	item := r.scrollTree(parentName, &r.root, true)
// 	if item == nil || item.childs == nil {
// 		return
// 	}
// 	for k, child := range item.childs {
// 		full := k
// 		if parentName != "" {
// 			full = parentName + "." + k
// 		}
// 		f(full, child.value)
// 	}
// }

// ForEachChildsParallel iterates immediate childs and calls f concurrently using a worker pool.
// Returns a channel that's closed when all work is done; caller can wait on it using <-done.
// func (r *RamCachedDB) ForEachChildsParallel(parentName string, f func(string, misc.DynamicVar), concurrency int) <-chan struct{} {
// 	done := make(chan struct{})
// 	if concurrency < 1 {
// 		concurrency = 1
// 	}

// 	r.mu.RLock()
// 	item := r.scrollTree(parentName, &r.root, true)
// 	if item == nil || item.childs == nil {
// 		r.mu.RUnlock()
// 		close(done)
// 		return done
// 	}

// 	// gather tasks
// 	tasks := make([]struct {
// 		full string
// 		val  misc.DynamicVar
// 	}, 0, len(item.childs))
// 	for k, child := range item.childs {
// 		full := k
// 		if parentName != "" {
// 			full = parentName + "." + k
// 		}
// 		tasks = append(tasks, struct {
// 			full string
// 			val  misc.DynamicVar
// 		}{full: full, val: child.value})
// 	}
// 	r.mu.RUnlock()

// 	var wg sync.WaitGroup
// 	taskCh := make(chan struct {
// 		full string
// 		val  misc.DynamicVar
// 	})

// 	// start workers
// 	for i := 0; i < concurrency; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for t := range taskCh {
// 				f(t.full, t.val)
// 			}
// 		}()
// 	}

// 	// feed tasks
// 	go func() {
// 		for _, t := range tasks {
// 			taskCh <- t
// 		}
// 		close(taskCh)
// 		wg.Wait()
// 		close(done)
// 	}()

// 	return done
// }

// dump expects r.mu to be locked. It writes the dump file to disk and returns any error.
func (r *RamCachedDB) dump() {
	r.log.Trace("Dumping RamCacheDB to disk...")
	text := r.dumpToStringLocked(&r.root, "")
	// ensure directory
	if err := os.MkdirAll(r.dataDir, 0o755); err != nil {
		r.log.Error("Error dumping RamCacheDB to disk: " + err.Error())
		return
	}
	path := filepath.Join(r.dataDir, "RamCacheDb.dump.txt")
	ret := os.WriteFile(path, []byte(text), 0o644)
	if ret != nil {
		r.log.Error("Error dumping RamCacheDB to disk: " + ret.Error())
		return
	}
	r.log.Trace("Finished dumping RamCacheDB to disk.")
}

func (r *RamCachedDB) dumpToStringLocked(current *ramNode, currentParentName string) string {
	ret := ""
	prefix := ""
	if currentParentName != "" {
		prefix = currentParentName + "."
	}
	for k, c := range current.childs {
		if c.value.GetString() != "" {
			ret += prefix + k + "=" + c.value.GetString() + "\n"
		}
		ret += r.dumpToStringLocked(c, prefix+k)
	}
	return ret
}

// Load reads the dump file and populates the in-memory cache. Existing nodes are preserved/extended.
func (r *RamCachedDB) load() error {
	path := filepath.Join(r.dataDir, "RamCacheDb.dump.txt")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	text := string(b)
	lines := strings.Split(text, "\n")
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		item := r.scrollTree(key, &r.root, false)
		if item != nil {
			item.value = misc.NewDynamicVar(val)
		}
	}
	r.pendingChanges.Store(false)
	return nil
}
