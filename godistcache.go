package godistcache

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mbarreca/godistcache/storage"
)

// This is the main cache object
type Cache struct {
	m       sync.RWMutex         // Used to prevent collisions
	items   map[string]CacheItem // Where the items are stored
	s3      *storage.S3
	exp     int64 // Default Expiration Time in Seconds
	crypt   cipher.BlockMode
	decrypt cipher.BlockMode
}

// This object is internally what exists in each item
type CacheItem struct {
	V interface{} // The item to store
	E int64       // Expiration timestamp in Unix UTC
}

// Creates a new cache
// exp -> The time, in seconds that you want default expiration, 0 is never expire
// ctx -> The context you want to provide for purposes of telemetry
func New(exp int64, ctx context.Context) (*Cache, error) {
	// Register the Cache Type with Gob
	gob.Register(CacheItem{})

	// Setup S3
	s3, err := storage.New(ctx)
	if err != nil {
		// Soft-fail
		fmt.Println(err)
	}
	// If "unlimited", set to 1000 years
	if exp == 0 {
		exp = 1000 * 365 * 24 * 60 * 60
	}
	// Check if Encryption is enabled
	crypt, decrypt, err := getEncryptionObjects()
	if err != nil {
		return nil, err
	}
	return &Cache{items: make(map[string]CacheItem), exp: exp, crypt: crypt, decrypt: decrypt, s3: s3}, nil
}

// Creates a new cache from a file in S3
// exp -> The time, in seconds that you want default expiration, 0 is never expire
// cacheKey -> The key you use in your S3 store that we'll pull from - DO NOT include the .godistcache extension
// ctx -> The context you want to provide for purposes of telemetry
func NewFromS3(exp int64, cacheKey string, ctx context.Context) (*Cache, error) {
	// Register the Cache Type with Gob
	gob.Register(CacheItem{})

	// Create new S3 Object
	s3, err := storage.New(ctx)
	if err != nil {
		return nil, err
	}
	// Download the File from the cache
	filePath, err := s3.S3Download(cacheKey + ".godistcache")
	if err != nil {
		return nil, err
	}
	// Create new cache
	c, err := New(0, ctx)
	if err != nil {
		return nil, err
	}
	// Load the entries in from the cache
	if err := c.LoadFromBinary(filePath); err != nil {
		return nil, err
	}
	// Delete the file and cleanup
	if err := os.Remove(filePath + ".godistcache"); err != nil {
		return nil, err
	}
	return c, nil
}

// This will set up a goroutine on the interval you select
// Interval - In seconds
// filePath -> The path to store the temporary file, the name comes from the ENV Variable GODISTCACHE_S3_OBJECT
func (c *Cache) SetupPersistToS3(interval int, filePath string) {
	if c.s3 == nil {
		panic("S3 isn't setup, can't setup persisting function")
	}
	for {
		go setupPersistToS3(c, filePath)
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// Goroutine to save the file, then upload to S3
// cache -> The cache you want to export
// filePath -> The path to store the temporary file, the name comes from the ENV Variable GODISTCACHE_S3_OBJECT
func setupPersistToS3(c *Cache, filePath string) {
	// Export to a file
	c.SaveToBinaryFile(filePath)
	// Upload it to S3
	c.s3.S3Upload(filePath, os.Getenv("GODISTCACHE_S3_OBJECT"))
	// Delete the file and cleanup
	if err := os.Remove(filePath + ".godistcache"); err != nil {
		fmt.Println(err)
	}
}

// Attempt to add an item to the cache
// key -> The key to lookup in the cache
// value -> The value to store in the cache
func (c *Cache) Put(key string, value any) {
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + c.exp}
	c.m.Unlock()
}

// Put an encrypted string in the cache
// key -> The key to lookup in the cache
// value -> The value to store in the cache
func (c *Cache) PutCrypt(key, value string) error {
	if c.crypt == nil {
		return errors.New("Encryption not set up")
	}
	v := c.encryptString(key)
	c.m.Lock()
	c.items[key] = CacheItem{V: v, E: time.Now().UTC().Unix() + c.exp}
	c.m.Unlock()
	return nil
}

// Put an encrypted string in the cache with custom expiration
// key -> The key to lookup in the cache
// value -> The value to store in the cache
// exp -> The expiration delay from now, in seconds
func (c *Cache) PutCryptExp(key, value string, exp int64) error {
	if c.crypt == nil {
		return errors.New("Encryption not set up")
	}
	v := c.encryptString(key)
	c.m.Lock()
	c.items[key] = CacheItem{V: v, E: time.Now().UTC().Unix() + exp}
	c.m.Unlock()
	return nil
}

// Attempt to add an item with a manual expiration offset (in seconds)
// key -> The key to lookup in the cache
// value -> The value to store in the cache
func (c *Cache) PutExp(key string, value any, exp int64) {
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + exp}
	c.m.Unlock()
}

// Add an item to the cache and send confirmation if successful, computationally more expensive (~10%)
// key -> The key to lookup in the cache
// value -> The value to store in the cache
func (c *Cache) PutSafe(key string, value any) bool {
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + c.exp}
	c.m.Unlock()
	// See if it exists
	valueNew, exists := c.Get(key)
	if exists {
		if value == valueNew {
			return true
		}
	}
	return false
}

// Add an item to the cache with custom expiration and send confirmation if successful. Computationally more expensive (~10%)
// key -> The key to lookup in the cache
// value -> The value to store in the cache
// exp -> The expiration delay from now, in seconds
func (c *Cache) PutSafeExp(key string, value any, exp int64) bool {
	// Set the item
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + exp}
	c.m.Unlock()
	valueNew, exists := c.Get(key)
	if exists {
		if value == valueNew {
			return true
		}
	}
	return false
}

// Attempt to get an item from the cache. Will return the item and a bool to indicate success
// key -> The key to lookup in the cache
func (c *Cache) Get(key string) (any, bool) {
	c.m.Lock()
	v := c.items[key]
	c.m.Unlock()
	// Check if the entry exists
	if v == (CacheItem{}) {
		return nil, false
	}
	// Check if the key has expired, if so delete
	if v.E < time.Now().UTC().Unix() {
		c.Delete(key)
		return nil, false
	}
	return v.V, true
}

// Attempt to get encrypted value from the cache. Will return the item and an error if unsuccessful
// key -> The key to lookup in the cache
func (c *Cache) GetCrypt(key string) (string, error) {
	c.m.Lock()
	v := c.items[key]
	c.m.Unlock()
	// Check if the entry exists
	if v == (CacheItem{}) {
		return "", errors.New("Entry doesn't exist")
	}
	// Check if the key has expired, if so delete
	if v.E < time.Now().UTC().Unix() {
		c.Delete(key)
		return "", errors.New("Entry is expired")
	}
	val, err := c.decryptString(v.V.(string))
	if err != nil {
		return "", err
	}
	return val, nil
}

// Delete an item from the cache
func (c *Cache) Delete(key string) {
	c.m.Lock()
	delete(c.items, key)
	c.m.Unlock()
}

// Delete an item from the cache with a check for safety, will return true if successful
// key -> The key to lookup in the cache
func (c *Cache) DeleteSafe(key string) bool {
	c.m.Lock()
	delete(c.items, key)
	v := c.items[key]
	c.m.Unlock()
	if v == (CacheItem{}) {
		return false
	}
	return true
}

// Returns the amount of items in the cache
func (c *Cache) Count() int {
	count := len(c.items)
	return count
}

// Tells you whether or not the item corresponding to the key exists
// key -> The key to lookup in the cache
func (c *Cache) Exists(key string) bool {
	c.m.Lock()
	val := c.items[key]
	c.m.Unlock()
	if val == (CacheItem{}) {
		return false
	}
	return true
}

// DANGEROUS - This will clear the cache
func (c *Cache) Clear() {
	c.m.Lock()
	clear(c.items)
	c.m.Unlock()
}

// This will convert the cache to a binary and save it to a file
// IMPORTANT -> Make sure to register all your structs with Gob before saving
// fileNamePath -> The path with the filename - DO NOT add the extension .godistcache
func (c *Cache) SaveToBinaryFile(filePathName string) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(c.items); err != nil {
		return err
	}
	// Check to see if the file exists
	if _, err := os.Stat(filePathName + ".godistcache"); err == nil {
		// If the file exists, delete it
		if err := os.Remove(filePathName + ".godistcache"); err != nil {
			return err
		}
	}
	// Write the file
	os.WriteFile(filePathName+".godistcache", buf.Bytes(), os.ModePerm)
	return nil
}

// This will load any .godistcache file into your cache
// filePathName -> The path with the filename - DO NOT add the extension .godistcache
// IMPORTANT -> Make sure to register all your structs with Gob before loading
func (c *Cache) LoadFromBinary(filePathName string) error {
	// Open the file
	file, err := os.ReadFile(filePathName + ".godistcache")
	if err != nil {
		return err
	}
	// Create the buffer
	buf := bytes.NewBuffer(file)
	dec := gob.NewDecoder(buf)
	m := make(map[string]CacheItem)
	// Decode the file
	if err := dec.Decode(&m); err != nil {
		return err
	}
	// Clear the cache and point it to the loaded map
	c.Clear()
	c.items = m
	return nil
}

/*
Encryption Functions
*/
// Get encryption objects for the cache to use
func getEncryptionObjects() (cipher.BlockMode, cipher.BlockMode, error) {
	if len(os.Getenv("GODISTCACHE_AES_CIPHER_KEY")) > 0 && len(os.Getenv("GODISTCACHE_AES_CIPHER_IV")) > 0 {
		// Enforce the length
		if len(os.Getenv("GODISTCACHE_AES_CIPHER_KEY")) != 32 && len(os.Getenv("GODISTCACHE_AES_CIPHER_IV")) != 16 {
			return nil, nil, errors.New("AES Key must be 32 characters and Cihper IV must be 16")
		}
		block, err := aes.NewCipher([]byte(os.Getenv("GODISTCACHE_AES_CIPHER_KEY")))
		if err != nil {
			return nil, nil, err
		}
		crypt := cipher.NewCBCEncrypter(block, []byte(os.Getenv("GODISTCACHE_AES_CIPHER_IV")))
		decrypt := cipher.NewCBCDecrypter(block, []byte(os.Getenv("GODISTCACHE_AES_CIPHER_IV")))
		return crypt, decrypt, nil
	}
	return nil, nil, nil
}

// Encrypt a string
func (c *Cache) encryptString(value string) string {
	paddedValue := pkcs5Padding([]byte(value), aes.BlockSize)
	cipherValue := make([]byte, len(paddedValue))
	c.crypt.CryptBlocks(cipherValue, paddedValue)
	return base64.StdEncoding.EncodeToString(cipherValue)
}

// Decrypt a string
func (c *Cache) decryptString(value string) (string, error) {
	cryptVal, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	c.decrypt.CryptBlocks(cryptVal, cryptVal)
	return string(pkcs5Unpad(cryptVal)), nil
}

// Pad according to PKCS#5 Standards
func pkcs5Padding(plain []byte, size int) []byte {
	pad := (size - len(plain)%size)
	paddedTxt := bytes.Repeat([]byte{byte(pad)}, pad)
	return append(plain, paddedTxt...)
}

// Unpad
func pkcs5Unpad(v []byte) []byte {
	unpad := int(v[len(v)-1])
	return v[:(len(v) - unpad)]
}
