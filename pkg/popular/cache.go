package popular

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/popular-packages.json
var popularData []byte

// Cache holds the popular packages cache
type Cache struct {
	index map[string]*Entry
}

var defaultCache *Cache

func init() {
	// Initialize cache - fail gracefully if data is missing or invalid
	var entries []Entry
	if len(popularData) == 0 {
		// No cache data embedded, skip initialization
		return
	}

	if err := json.Unmarshal(popularData, &entries); err != nil {
		// Invalid cache data, skip initialization
		// Tool will work without popular cache
		return
	}

	// Build index for O(1) lookups
	index := make(map[string]*Entry, len(entries))
	for i := range entries {
		index[entries[i].Package] = &entries[i]
	}

	defaultCache = &Cache{
		index: index,
	}
}

// Lookup retrieves a package entry from the popular cache
// Returns the entry and true if found, nil and false otherwise
func Lookup(pkg string) (*Entry, bool) {
	if defaultCache == nil {
		return nil, false
	}
	entry, ok := defaultCache.index[pkg]
	return entry, ok
}

// Size returns the number of entries in the cache
func Size() int {
	if defaultCache == nil {
		return 0
	}
	return len(defaultCache.index)
}

// IsLoaded returns true if the popular cache is loaded
func IsLoaded() bool {
	return defaultCache != nil
}

// Stats returns cache statistics as a string
func Stats() string {
	if !IsLoaded() {
		return "Popular cache: not loaded"
	}
	return fmt.Sprintf("Popular cache: %d packages", Size())
}
