package godistcache

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"os"
	"sync"
	"time"
)

// This is the main cache object
type Cache struct {
	m       sync.RWMutex         // Used to prevent collisions
	items   map[string]CacheItem // Where the items are stored
	exp     int64                // Default Expiration Time in Seconds
	crypt   cipher.BlockMode
	decrypt cipher.BlockMode
}

// This object is internally what exists in each item
type CacheItem struct {
	V interface{} // The item to store
	E int64       // Expiration timestamp in Unix UTC
}

// Creates a new cache
// size - The amount of elements you want to store
// exp  - The time, in seconds that you want default expiration, 0 is never expire
func New(exp int64) (*Cache, error) {
	// If "unlimited", set to 1000 years
	if exp == 0 {
		exp = 1000 * 365 * 24 * 60 * 60
	}
	if len(os.Getenv("GODIST_AES_CIPHER_KEY")) > 0 && len(os.Getenv("GODIST_AES_CIPHER_IV")) > 0 {
		// Enforce the length
		if len(os.Getenv("GODIST_AES_CIPHER_KEY")) != 32 && len(os.Getenv("GODIST_AES_CIPHER_IV")) != 16 {
			return nil, errors.New("AES Key must be 32 characters and Cihper IV must be 16")
		}
		block, err := aes.NewCipher([]byte(os.Getenv("GODIST_AES_CIPHER_KEY")))
		if err != nil {
			return nil, err
		}
		crypt := cipher.NewCBCEncrypter(block, []byte(os.Getenv("GODIST_AES_CIPHER_IV")))
		decrypt := cipher.NewCBCDecrypter(block, []byte(os.Getenv("GODIST_AES_CIPHER_IV")))
		return &Cache{items: make(map[string]CacheItem), exp: exp, crypt: crypt, decrypt: decrypt}, nil
	}
	return &Cache{items: make(map[string]CacheItem), exp: exp, crypt: nil}, nil
}

// Attempt to add an item to the cache
func (c *Cache) Put(key string, value any) {
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + c.exp}
	c.m.Unlock()
}

// Put an encrypted string in the cache
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

// Put an encrypted string in the cache
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
func (c *Cache) PutExp(key string, value any, exp int64) {
	c.m.Lock()
	c.items[key] = CacheItem{V: value, E: time.Now().UTC().Unix() + exp}
	c.m.Unlock()
}

// Add an item to the cache and send confirmation if successful,
// Computationally more expensive (~20%)
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

// Add an item to the cache with custom expiration and send confirmation if successful,
// Computationally more expensive (~20%)
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

// Attempt to get an item from the cache
// Will return the item and a bool to indicate success
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

// Attempt to get encrypted value from the cache
// Will return the item and an error if unsuccessful
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

// Delete an item from the cache with a check for safety
// Will return true if successful
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
// The function will automatically add the extension .godistcache
// IMPORTANT -> Make sure to register all your structs with Gob before saving
func (c *Cache) SaveToBinaryFile(filepath string) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(c.items); err != nil {
		return err
	}
	os.WriteFile(filepath+".godistcache", buf.Bytes(), os.ModePerm)
	return nil
}

// This will load any .godistcache file into your cache
// The function will automatically add the extension .godistcache
// IMPORTANT -> Make sure to register all your structs with Gob before loading
func (c *Cache) LoadFromBinary(filepath string) error {
	// Open the file
	file, err := os.ReadFile(filepath + ".godistcache")
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

func (c *Cache) encryptString(value string) string {
	paddedValue := pkcs5Padding([]byte(value), aes.BlockSize)
	cipherValue := make([]byte, len(paddedValue))
	c.crypt.CryptBlocks(cipherValue, paddedValue)
	return base64.StdEncoding.EncodeToString(cipherValue)
}

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
