# Theine(WIP)
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
- [Benchmarks](#benchmarks)
  * [throughput benchmark](#throughput-benchmark)
  * [hit ratios](#hit-ratios)

## Requirements
Go 1.19+

## Installation
```
go get github.com/Yiling-J/theine-go

```

## API

Key should be **comparable**, and value can be any.

```Go
import "github.com/Yiling-J/theine-go"

// key type string, value type string, max size 1000
client, err := theine.New[string, string](1000)
if err != nil {
	panic(err)
}

// set, key foo, value bar, cost 1
success := client.Set("foo", "bar", 1)

// set with ttl
success = client.SetWithTTL("foo", "bar", 1, 1*time.Second)

// get
value, ok := client.Get("foo")

// remove
client.Delete("foo")

```
## Benchmarks

### throughput benchmark

```
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz

BenchmarkGetTheineParallel-12           32432190                36.39 ns/op            0 B/op          0 allocs/op
BenchmarkGetRistrettoParallel-12        63978058                18.86 ns/op           17 B/op          1 allocs/op
BenchmarkSetTheineParallel-12           20791834                84.49 ns/op            0 B/op          0 allocs/op
BenchmarkSetRistrettoParallel-12        23354626                65.53 ns/op          116 B/op          3 allocs/op
BenchmarkZipfTheineParallel-12          14771362                74.72 ns/op            1 B/op          0 allocs/op
BenchmarkZipfRistrettoParallel-12       21031435                61.82 ns/op          100 B/op          3 allocs/op
```

### hit ratios

ristretto v0.1.1: https://github.com/dgraph-io/ristretto

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
