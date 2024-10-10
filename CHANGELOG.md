## 0.5.0 (2024-10-10)

### API Changes:
- The NVM secondary cache has been moved to a separate package: https://github.com/Yiling-J/theine-nvm.

### Enhancements:
- Reduced `Set` allocations, making Theine zero allocation (amortized).
- Improved read performance slightly by utilizing a cached `now` value.
- Fixed race conditions in cost (weight) updates that could cause inaccurate policy cost.
- Added benchmarks for different `GOMAXPROC` values in the README.

## 0.4.1 (2024-08-22)

### Enhancements:
* Use x/sys/cpu cacheline size by @Yiling-J in https://github.com/Yiling-J/theine-go/pull/43
* Add Size method on cache by @nlachfr in https://github.com/Yiling-J/theine-go/pull/41
* Accurate hits/misses counter by @Yiling-J in https://github.com/Yiling-J/theine-go/pull/44
* Add stats API by @Yiling-J in https://github.com/Yiling-J/theine-go/pull/45
