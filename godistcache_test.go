package godistcache

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// GoDistCache Testing and Benchmarking

var amountOfRuns = 1000

type Object struct {
	One   string  `json:"one"`
	Two   int     `json:"two"`
	Three float64 `json:"three"`
}

func TestGoDistCachePut(t *testing.T) {

	// Create Cache
	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	// Testing Loading the cache
	cacheLoad(c, s, objs)
	elapsed := time.Since(start)
	putsTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Cache PUT Requests took %s, requests per second is %f", amountOfRuns, elapsed, putsTime)
}

func TestGoDistCacheSafePut(t *testing.T) {

	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	// Testing Loading the cache
	cacheLoadSafe(c, s, objs)
	elapsed := time.Since(start)
	putsSafeTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Cache Safe PUT Requests took %s, requests per second is %f", amountOfRuns, elapsed, putsSafeTime)
}

func TestGoDistCacheGet(t *testing.T) {

	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}

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

	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}
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

func TestGoDistCacheSaveLoad(t *testing.T) {
	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}

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
	c2, err := New(0, context.Background())
	if err != nil {
		t.Fatal(err)
	}

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

func TestGoDistCacheCrypt(t *testing.T) {

	c, s, _, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	// Load the Cache
	cacheLoadCrypt(c, s)
	elapsed := time.Since(start)
	putsTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Crypt Cache PUT Requests took %s, requests per second is %f", amountOfRuns, elapsed, putsTime)

	// Wait for value to pass through buffers
	time.Sleep(50 * time.Millisecond)

	// Get Data from the Cache and verify
	start = time.Now()
	for i := 0; i < amountOfRuns; i++ {
		v, err := c.GetCrypt(s[i])
		if err != nil || v != s[i] {
			t.Fatalf("Error: Crypt Cache Get doesn't match input values. %v", err)
		}
	}
	elapsed = time.Since(start)
	getsTime := (1 / time.Duration.Seconds(elapsed)) * float64(amountOfRuns)
	t.Logf("Simulating %d Crypt Cache GET Requests took %s, requests per second is %f", amountOfRuns, elapsed, getsTime)
}

func TestGoCacheSyncToS3(t *testing.T) {

	c, s, objs, err := cacheCreateWithObjects()
	if err != nil {
		t.Fatal(err)
	}
	// Get current working directory
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Load the cache
	cacheLoad(c, s, objs)

	// Setup persistence
	go c.SetupPersistToS3(5, pwd)

	if err := c.SaveToBinaryFile(pwd + "test"); err != nil {
		t.Fatal(err)
	}
	c.s3.S3Upload(pwd+"test", "test")

	// Wait for the S3 upload
	time.Sleep(time.Second * 25)

	// Load from S3
	c2, err := NewFromS3(0, "test", context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Test if its loaded properly
	for i := 0; i < amountOfRuns; i++ {
		v, e := c2.Get(s[i])
		v2, e2 := c.Get(s[i])
		if v != v2 && e != e2 {
			t.Fatalf("Failed reading Safe PUT on Index: %v", i)
		}
	}
	t.Log("Successfully loaded the cache, saved it to S3, loaded it from S3 and confirmed the results are the same")
}

func cacheCreateWithObjects() (*Cache, []string, []Object, error) {
	os.Setenv("GODISTCACHE_AES_CIPHER_KEY", "cWlW2XekajJmuZqwAFNJTXqJ28YjiiP1")
	os.Setenv("GODISTCACHE_AES_CIPHER_IV", "Jh0VdNhFATWOPxvM")
	gob.Register(Object{})
	gob.Register(CacheItem{})

	// Create Cache
	c, err := New(0, context.Background())
	if err != nil {
		return nil, nil, nil, err
	}

	s, objs := createObjects()
	return c, s, objs, nil
}

func cacheCreateWithObjectsFromS3(cacheKey string) (*Cache, []string, []Object, error) {
	os.Setenv("GODISTCACHE_AES_CIPHER_KEY", "cWlW2XekajJmuZqwAFNJTXqJ28YjiiP1")
	os.Setenv("GODISTCACHE_AES_CIPHER_IV", "Jh0VdNhFATWOPxvM")

	// Create Cache
	c, err := NewFromS3(0, cacheKey, context.Background())
	if err != nil {
		return nil, nil, nil, err
	}
	s, objs := createObjects()
	return c, s, objs, nil
}

func createObjects() ([]string, []Object) {
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
	return s, objs
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

func cacheLoadCrypt(c *Cache, s []string) {
	for i := 0; i < amountOfRuns; i++ {
		c.PutCrypt(s[i], s[i])
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
