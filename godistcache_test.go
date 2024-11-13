package godistcache

import (
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// GoDistCache Testing and Benchmarking

var amountOfRuns = 1000000

type Object struct {
	One   string  `json:"one"`
	Two   int     `json:"two"`
	Three float64 `json:"three"`
}

func TestGoDistCachePut(t *testing.T) {

	// Create Cache
	c, s, objs := cacheCreateWithObjects()
	start := time.Now()
	// Testing Loading the cache
	cacheLoad(c, s, objs)
	elapsed := time.Since(start)
	putsTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Cache PUT Requests took %s, requests per second is %f", amountOfRuns, elapsed, putsTime)
}

func TestGoDistCacheSafePut(t *testing.T) {

	c, s, objs := cacheCreateWithObjects()
	start := time.Now()
	// Testing Loading the cache
	cacheLoadSafe(c, s, objs)
	elapsed := time.Since(start)
	putsSafeTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Cache Safe PUT Requests took %s, requests per second is %f", amountOfRuns, elapsed, putsSafeTime)
}

func TestGoDistCacheGet(t *testing.T) {

	c, s, objs := cacheCreateWithObjects()

	// Load the Cache
	cacheLoad(c, s, objs)

	// Wait for value to pass through buffers
	time.Sleep(50 * time.Millisecond)

	// Get Data from the Cache
	start := time.Now()
	for i := 0; i < amountOfRuns; i++ {
		c.Get(s[i])
	}
	elapsed := time.Since(start)
	getsTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Cache GET Requests took %s, requests per second is %f", amountOfRuns, elapsed, getsTime)
}

// GoDistCache Tests
func TestGoDistCache(t *testing.T) {

	c, s, objs := cacheCreateWithObjects()
	// Test PUT
	cacheLoad(c, s, objs)

	// Wait for value to pass through buffers
	time.Sleep(50 * time.Millisecond)

	// Check Count
	if c.Count() != amountOfRuns {
		t.Fatalf("Amount of Cache Items != Amount Counted")
	}
	if err := cacheCheckLoadedProperly(c, s, objs); err != nil {
		t.Fatal(err)
	}
	c.Clear()

	// Check Count
	if c.Count() != 0 {
		t.Fatalf("Clear cache doesn't work")
	}

	// Load the Cache with PutSafe
	cacheLoadSafe(c, s, objs)

	// Wait for value to pass through buffers
	time.Sleep(50 * time.Millisecond)

	// Check Count
	if c.Count() != amountOfRuns {
		t.Fatalf("Amount of Cache Items != Amount Counted")
	}

	// Test if its loaded properly
	if err := cacheCheckLoadedProperly(c, s, objs); err != nil {
		t.Fatal(err)
	}

	// Delete Test
	if err := cacheDelete(c, s); err != nil {
		t.Fatal(err)
	}
}

// GoDistCache Tests
func TestGoDistCacheSaveLoad(t *testing.T) {
	gob.Register(Object{})
	gob.Register(CacheItem{})
	c, s, objs := cacheCreateWithObjects()
	// Test PUT
	cacheLoad(c, s, objs)
	// Get current working directory
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fpwd := pwd + "/test"
	// Benchmark Save to JSON
	start := time.Now()
	err = c.SaveToBinaryFile(fpwd)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	saveTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating Saving %d Items took %s, items per second is %f", amountOfRuns, elapsed, saveTime)
	// Create cache copy
	c2 := New(uint32(amountOfRuns), 0)

	// Benchmark Load from JSON
	start = time.Now()
	err = c2.LoadFromBinary(fpwd)
	if err != nil {
		t.Fatal(err)
	}
	elapsed = time.Since(start)
	loadTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating Loading %d Items took %s, items per second is %f", amountOfRuns, elapsed, loadTime)
	// Test if its loaded properly
	for i := 0; i < amountOfRuns; i++ {
		v, e := c2.Get(s[i])
		if !e || v != objs[i] {
			t.Fatalf("Failed reading Safe PUT on Index: %v", i)
		}
	}

	// Make sure it loaded properly
	if err := cacheCheckLoadedProperly(c2, s, objs); err != nil {
		t.Fatal(err)
	}

	// Clean Up
	if err := os.Remove(pwd + "/test.godistcache"); err != nil {
		t.Fatal(err)
	}
}
func cacheCreateWithObjects() (*Cache, []string, []Object) {
	// Create Cache
	c := New(uint32(amountOfRuns), 0)

	// Create objects
	var objs []Object
	for i := 0; i < amountOfRuns+1; i++ {
		var obj Object
		obj.One = "One"
		obj.Two = 2
		obj.Three = 3.3333
		objs = append(objs, obj)
	}
	// Create strings
	var s []string
	for i := 0; i < amountOfRuns+1; i++ {
		s = append(s, strconv.Itoa(i))
	}
	return c, s, objs
}

func cacheCheckLoadedProperly(c *Cache, s []string, objs []Object) error {
	// Loop through cache and make sure it equals our inputs
	for i := 0; i < amountOfRuns; i++ {
		v, e := c.Get(s[i])
		if !e || v != objs[i] {
			return errors.New(fmt.Sprintf("Failed reading Get on Index: %v", i))
		}
	}
	return nil
}

func cacheLoad(c *Cache, s []string, objs []Object) {
	for i := 0; i < amountOfRuns; i++ {
		c.Put(s[i], objs[i])
	}
}
func cacheLoadSafe(c *Cache, s []string, objs []Object) {
	for i := 0; i < amountOfRuns; i++ {
		c.PutSafe(s[i], objs[i])
	}
}
func cacheDelete(c *Cache, s []string) error {
	for i := 0; i < amountOfRuns; i++ {
		c.Delete(s[i])
		count := c.Count()
		countShould := amountOfRuns - (i + 1)
		if count != countShould {
			return errors.New(fmt.Sprintf("Failed DELETE on Index: %v, Count: %v, Count should be: %v", i, count, countShould))
		}
	}
	return nil
}
