# Theine
[![codecov](https://codecov.io/gh/Yiling-J/theine-go/branch/main/graph/badge.svg?token=E1HJLJH07V)](https://codecov.io/gh/Yiling-J/theine-go)

High performance in-memory cache inspired by [Caffeine](https://github.com/ben-manes/caffeine).


- Good performance
- Support for Generics
- High hit ratio with adaptive [W-TinyLFU](https://arxiv.org/pdf/1512.00727.pdf) eviction policy
- Expired data are removed automatically using [hierarchical timer wheel](http://www.cs.columbia.edu/~nahum/w6998/papers/ton97-timing-wheels.pdf)

  > TTL must be considered in in-memory caching because
it limits the effective (unexpired) working set size. Efficiently removing expired objects from cache needs to be
prioritized over cache eviction. - [A large scale analysis of hundreds of in-memory
cache clusters at Twitter](https://www.usenix.org/system/files/osdi20-yang.pdf)
- Simple API

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [API](#api)
- [Hybrid Cache](#hybrid-cache)
- [Cache Persistence](#cache-persistence)
- [Benchmarks](#benchmarks)
  * [throughput](#throughput)
  * [hit ratios](#hit-ratios)
- [Tips](#tips)
- [Support](#support)

## Requirements
Go 1.19+

## Installation
```
go get github.com/Yiling-J/theine-go
```

## API

**Builder API**

Theine provides two types of client, simple cache and loading cache. Both of them are initialized from a builder. The difference between simple cache and loading cache is: loading cache's Get method will compute the value using loader function when there is a miss, while simple cache client only return false and do nothing.

Loading cache uses [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) to prevent concurrent loading to same key(thundering herd).

simple cache:

```GO
import "github.com/Yiling-J/theine-go"

// key type string, value type string, max size 1000
// max size is the only required configuration to build a client
client, err := theine.NewBuilder[string, string](1000).Build()
if err != nil {
	panic(err)
}

// builder also provide several optional configurations
// you can chain them together and call build once
// client, err := theine.NewBuilder[string, string](1000).Cost(...).Doorkeeper(...).Build()

// or create builder first
builder := theine.NewBuilder[string, string](1000)

// dynamic cost function based on value
// use 0 in Set will call this function to evaluate cost at runtime
builder.Cost(func(v string) int64 {
		return int64(len(v))
})

// doorkeeper
// doorkeeper will drop Set if they are not in bloomfilter yet
// this can improve write performance, but may lower hit ratio
builder.Doorkeeper(true)

// removal listener, this function will be called when entry is removed
// RemoveReason could be REMOVED/EVICTED/EXPIRED
// REMOVED: remove by API
// EVICTED: evicted by Window-TinyLFU policy
// EXPIRED: expired by timing wheel
builder.RemovalListener(func(key K, value V, reason theine.RemoveReason) {})

```
loading cache:

```go
import "github.com/Yiling-J/theine-go"

// loader function: func(ctx context.Context, key K) (theine.Loaded[V], error)
// Loaded struct should include cache value, cost and ttl, which required by Set method
client, err := theine.NewBuilder[string, string](1000).BuildWithLoader(
	func(ctx context.Context, key string) (theine.Loaded[string], error) {
		return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
	},
)
if err != nil {
	panic(err)
}

```
Other builder options are same as simple cache(cost, doorkeeper, removal listener).


**Client API**

```Go
// set, key foo, value bar, cost 1
// success will be false if cost > max size
success := client.Set("foo", "bar", 1)
// cost 0 means using dynamic cost function
// success := client.Set("foo", "bar", 0)

// set with ttl
success = client.SetWithTTL("foo", "bar", 1, 1*time.Second)

// get(simple cache version)
value, ok := client.Get("foo")

// get(loading cache version)
value, err := client.Get(ctx, "foo")

// remove
client.Delete("foo")

// iterate key/value in cache and apply custom function
// if function returns false, range stops the iteration
client.Range(func(key, value int) bool {
	return true
})

// close client, set hashmaps in shard to nil and close all goroutines
client.Close()

```

## Hybrid Cache

HybridCache feature enables Theine to extend the DRAM cache to NVM. With HybridCache, Theine can seamlessly move Items stored in cache across DRAM and NVM as they are accessed. Using HybridCache, you can shrink your DRAM footprint of the cache and replace it with NVM like Flash. This can also enable you to achieve large cache capacities for the same or relatively lower power and dollar cost.

#### Design
Hybrid Cache is inspired by CacheLib's HybridCache. See [introduction](https://cachelib.org/docs/Cache_Library_User_Guides/HybridCache) and [architecture](https://cachelib.org/docs/Cache_Library_Architecture_Guide/hybrid_cache) from CacheLib's guide.

When you use HybridCache, items allocated in the cache can live on NVM or DRAM based on how they are accessed. Irrespective of where they are, **when you access them, you always get them to be in DRAM**.

Items start their lifetime on DRAM. As an item becomes cold it gets evicted from DRAM when the cache is full. Theine spills it to a cache on the NVM device. Upon subsequent access through `Get()`, if the item is not in DRAM, theine looks it up in the HybridCache and if found, moves it to DRAM. When the HybridCache gets filled up, subsequent insertions into the HybridCache from DRAM will throw away colder items from HybridCache.

Same as CacheLib, Theine hybrid cache also has **BigHash** and **Block Cache**, it's highly recommended to read the CacheLib architecture design before using hybrid cache, here is a simple introduction of these 2 engines(just copy from CacheLib):

-   **BigHash**  is effectively a giant fixed-bucket hash map on the device. To read or write, the entire bucket is read (in case of write, updated and written back). Bloom filter used to reduce number of IO. When bucket is full, items evicted in FIFO manner. You don't pay any RAM price here (except Bloom filter, which is 2GB for 1TB BigHash, tunable). 
-   **Block Cache**, on the other hand, divides device into equally sized regions (16MB, tunable) and fills a region with items of same size class, or, in case of log-mode fills regions sequentially with items of different size. Sometimes we call log-mode “stack alloc”. BC stores compact index in memory: key hash to offset. We do not store full key in memory and if collision happens (super rare), old item will look like evicted. In your calculations, use 12 bytes overhead per item to estimate RAM usage. For example, if your average item size is 4KB and cache size is 500GB you'll need around 1.4GB of memory.

#### Using Hybrid Cache

To use HybridCache, you need to create a nvm cache with NvmBuilder. NewNvmBuilder require 2 params, first is cache file name, second is cache size in bytes. Theine will use direct I/O to read/write file.

```go
nvm, err := theine.NewNvmBuilder[int, int]("cache", 150<<20).[settings...].Build()
```

Then enable hybrid mode in your Theine builder.
```go
client, err := theine.NewBuilder[int, int](100).Hybrid(nvm).Build()
```

#### NVM Builder Settings

All settings are optional, unless marked as "Required".

* **[Common]** `BlockSize` default 4096
    Device block size in bytes (minimum IO granularity).
* **[Common]** `KeySerializer` default JsonSerializer
    KeySerializer is used to marshal/unmarshal between your key type and bytes.
    ```go
    type Serializer[T any] interface {
	    Marshal(v T) ([]byte, error)
	    Unmarshal(raw []byte, v *T) error
    }
    ```
* **[Common]** `ValueSerializer` default JsonSerializer
    ValueSerializer is used to marshal/unmarshal between your value type and bytes. Same interface as KeySerializer.
* **[BlockCache]** `ErrorHandler` default do nothing
    Nvm cache flush data to disk async, so errors will be handled by this error handler.
* **[BlockCache]** `RegionSize` default 16 << 20 (16 mb)
    Region size in bytes.
* **[BlockCache]** `CleanRegionSize` default 3
    How many regions do we reserve for future writes. Set this to be equivalent to your per-second write rate. It should ensure your writes will not have to retry to wait for a region reclamation to finish.
* **[BigHash]** `BucketSize` defalut 4 << 10 (4 kb)
    Bucket size in bytes.
* **[BigHash]** `BigHashPct` default 10
    Percentage of space to reserve for BigHash. Set the percentage > 0 to enable BigHash. The remaining part is for BlockCache. The value has to be in the range of [0, 100]. Set to 100 will disable block cache.
* **[BigHash]** `BigHashMaxItemSize` default (bucketSize - 80)
    Maximum size of a small item to be stored in BigHash. Must be less than (bucket size - 80).

#### Hybrid Mode Settings

After you call `Hybrid(...)` in a cache builder. Theine will convert current builder to hybrid builder. Hybrid builder has several settings.

* `Workers` defalut 2
    Theine evicts entries in a separate policy goroutinue, but insert to NVM can be done parallel. To make this work, Theine send evicted entries to workers, and worker will sync data to NVM cache. This setting controls how many workers are used to sync data.
	
* `AdmProbability` defalut 1
    This is a admission policy for nvm cache. When entries are evicted from DRAM cache, this policy will be used to control the insert percentage. 1 means all entries evicted from DRAM will be insert into NVM. Value should be in the range of [0, 1].

#### Limitations
- Cache Persistence is not currently supported, but it may be added in the future. You can still use the Persistence API in a hybrid-enabled cache, but only the DRAM part of the cache will be saved or loaded.
- The removal listener will only receive REMOVED events, which are generated when an entry is explicitly removed by calling the Delete API.


## Cache Persistence
Theine supports persisting the cache into `io.Writer` and restoring from `io.Reader`. [Gob](https://pkg.go.dev/encoding/gob) is used to encode/decode data, so **make sure your key/value can be encoded by gob correctly first** before using this feature.

#### API
```go
func (c *Cache[K, V]) SaveCache(version uint64, writer io.Writer) error
func (c *Cache[K, V]) LoadCache(version uint64, reader io.Reader) error
```
**- Important:** please `LoadCache` immediately after client created, or existing entries' TTL might be affected.

#### Example:
```go
// save
f, err := os.Create("test")
err := client.SaveCache(0, f)
f.Close()

// load
f, err = os.Open("test")
require.Nil(t, err)
newClient, err := theine.NewBuilder[int, int](100).Build()
// load immediately after client created
err = newClient.LoadCache(0, f)
f.Close()
```
Version number must be same when saving and loading, or `LoadCache` will return `theine.VersionMismatch` error. You can change the version number when you want to ignore persisted cache.
```go
err := newClient.LoadCache(1, f)
// VersionMismatch is a global variable
if err == theine.VersionMismatch {
	// ignore and skip loading
} else if err != nil {
	// panic error
}
```

#### Details
When persisting cache, Theine roughly do:
- Store version number.
- Store clock(used in TTL).
- Store frequency sketch.
- Store entries one by one in protected LRU in most-recently:least-recently order.
- Store entries one by one in probation LRU in most-recently:least-recently order.
- Loop shards and store entries one by one in each shard deque.

When loading cache, Theine roughly do:
- Load version number, compare to current version number.
- Load clock.
- Load frequency sketch.
- Load protected LRU and insert entries back to new protected LRU and shards/timingwheel, expired entries will be ignored. Because cache capacity may change, this step will stop if max protected LRU size reached.
- Load probation LRU and insert entries back to new probation LRU and shards/timingwheel, expired entries will be ignored, Because cache capacity may change, this step will stop if max probation LRU size reached.
- Load deque entries and insert back to shards, expired entries will be ignored.

Theine will save checksum when persisting cache and verify checksum first when loading.

## Benchmarks

Source: https://github.com/Yiling-J/go-cache-benchmark-plus

This repo includes reproducible throughput/hit-ratios benchmark code, you can also test your own cache package with it.
	
### throughput

```
goos: darwin
goarch: amd64
pkg: github.com/Yiling-J/go-cache-benchmark-plus
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkGetParallel/theine-12          40604346                28.72 ns/op            0 B/op          0 allocs/op
BenchmarkGetParallel/ristretto-12       60166238                23.50 ns/op           17 B/op          1 allocs/op
BenchmarkSetParallel/theine-12          16067138                67.55 ns/op            0 B/op          0 allocs/op
BenchmarkSetParallel/ristretto-12       12830085                79.30 ns/op          116 B/op          3 allocs/op
BenchmarkZipfParallel/theine-12         15908767                70.07 ns/op            0 B/op          0 allocs/op
BenchmarkZipfParallel/ristretto-12      17200935                80.05 ns/op          100 B/op          3 allocs/op
```

### hit ratios

ristretto v0.1.1: https://github.com/dgraph-io/ristretto
> from Ristretto [README](https://github.com/dgraph-io/ristretto#hit-ratios), the hit ratio should be higher. But I can't reproduce their benchmark results. So I open an issue: https://github.com/dgraph-io/ristretto/issues/336

golang-lru v2.0.2: https://github.com/hashicorp/golang-lru

**zipf**

![hit ratios](benchmarks/results/zipf.png)
**search**

This trace is described as "disk read accesses initiated by a large commercial search engine in response to various web search requests."

![hit ratios](benchmarks/results/s3.png)
**database**

This trace is described as "a database server running at a commercial site running an ERP application on top of a commercial database."

![hit ratios](benchmarks/results/ds1.png)
**Scarabresearch database trace**

Scarabresearch 1 hour database trace from this [issue](https://github.com/ben-manes/caffeine/issues/106)

![hit ratios](benchmarks/results/scarab1h.png)
**Meta anonymized trace**

Meta shared anonymized trace captured from large scale production cache services, from [cachelib](https://cachelib.org/docs/Cache_Library_User_Guides/Cachebench_FB_HW_eval/#running-cachebench-with-the-trace-workload)

![hit ratios](benchmarks/results/meta.png)

## Tips
- If your key size is very large, you may consider using a struct with 2 hashes instead:
```go
type hashKey struct {
	key uint64
	conflict uint64
}
```
This is how Ristretto handle keys. But keep in mind that even though the collision rate is very low, it's still possible.

## Support
Open an issue, ask question in discussions or join discord channel: https://discord.gg/StrgfPaQqE 
