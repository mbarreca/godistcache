# godistcache

<div align="center">

Godistcache is a general object performance oriented cache that offers saving/loading to/from a file and built-in AES encryption.

In the current cache landscape you either have something centralized (like Redis or Ignite - which can be distributed) or entirely local. Theres nothing with the speed of local and power of centralized, which was the inspiration for this. The methodology of how you want to sync is left up to the user. Currently I'm providing a write to a Binary file. I have experimented with JSON, it works if you only use built in types, so it hasn't been added to the library yet.

The cache is quite performant (based on my tests, 1M Entries, M1 Pro Mac, results may vary):

GET: 5M/s\
PUT: 3.5M/s\
Safe PUT: 3M/s\
Encrypted GET: 4M/s\
Encrypted PUT: 2.25M/s\
Save to file: 2.75M\
Load to file: 2M/s\

This cache has zero external depedencies, only utilized built in golang libraries

[![GoDoc][doc-img]][doc]

<div align="left">

## Installation

<div align="center">

`go get github.com/mbarreca/godistcache`

<div align="left">

Godistcache was built and tested with Go 1.23, it may still work with prior versions however it has not been tested so use at your own risk.

## Register Objects

In order for the export and import to function correctly with structs, you need to register all your struct types with Gob. In the example below I've provided how you do this. It's simply a matter of `gob.register(structName{})`

## Environment Variables

You'll need to set the following environment variables in order to provide the correct values to the system.

```
// Set these in order to setup encryption
// Must be 32 characters
GODIST_AES_CIPHER_KEY="KwSHE3K0jrMB6MSiQsD9DBLxZx23FHFA"
// Must be 16 characters
GODIST_AES_CIPHER_IV="uGwDbXeAWoihBYq1"
```

## Example Usage
```
package main

import (
	"encoding/gob"
	"fmt"
	"os"

	"github.com/mbarreca/godistcache"
)

type Object struct {
	One   string
	Two   int
	Three float64
}

func main() {
	// Gob Registeration for Save/Load
	gob.Register(Object{})

	// Create Cache Object
	cache, err := godistcache.New(0)
	if err != nil {
		panic(err)
	}

	// Create sample object
	a := Object{One: "One", Two: 2, Three: 3.3}
	b := Object{One: "One", Two: 2, Three: 3.3}
	c := Object{One: "One", Two: 2, Three: 3.3}

	// Put
	cache.Put("a", a)
	success := cache.PutSafe("b", b)
	if !success {
		fmt.Printf("Cache failed to Put object: %v", b)
	}
	cache.PutExp("c", 86400)

	// Get
	a, success = cache.Get("a")
	if !success {
		fmt.Printf("Cache failed to Get object with key: %v", "b")
	}
	fmt.Printf("Value for Key A: %v", a)

	// Get current working directory
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	fpwd := pwd + "/test"

	// Save to File
	err = cache.SaveToBinaryFile(fpwd)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	// Create a new cache
	cacheNew := godistcache.New(0)

	// Load from the file we saved earlier
	err = cacheNew.LoadFromFile(fpwd)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	// Get from the new cache
	a, success = cacheNew.Get("a")
	if !success {
		fmt.Printf("Cache failed to Get object with key: %v", "b")
	}
	fmt.Printf("Value for Key A: %v", a)

	// Success!
}
```
## Expiration

The goal of this library is to have strong performance. RAM is cheap, compute is not. Expiration can either happen on an interval or programmatically. So we check on each get if the key has expired, if so we delete it. We also only save items that aren't expired (though the cache isn't cleared to save, again performance).

## OpenTelemetry

Opentelemetry is intentially left out due to performance reasons. It is on the roadmap as something to potentially include as an option, but there will have to be sufficient warning that this is not a fast process.

## Testing
`
go test -v
`

## Roadmap

- S3 Sync
- Better Performance in High Load Situations (10M+ entries)
- Built in types - JSON Export
- Telemetry Support

## License

This is licensed under the Apache 2.0 License

[doc]: https://pkg.go.dev/github.com/mbarreca/godistcache
[doc-img]: https://pkg.go.dev/badge/github.com/mbarreca/godistcache
