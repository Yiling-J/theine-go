package stats

import (
	"sync/atomic"

	"github.com/Yiling-J/theine-go/internal/clock"
)

type CacheStatsType int

const (
	// total
	NumCacheGets CacheStatsType = iota
	NumCacheGetMiss
	NumCacheGetExpires
	NumCacheEvictions
	NumItems

	// nvm
	NumNvmGets
	NumNvmGetMiss
	NumNvmGetExpires
	NumNvmEvictions

	// counter end
	counterStatsEnd

	// percentile stats
	LoadingCacheLatency
	NvmLookupLatency
	NvmInsertLatency
	NvmRemoveLatency

	// percentile stats end
	percentileStatsEnd
)

type CacheStats struct {
	NumItems           uint64
	NumCacheGets       uint64
	NumCacheGetMiss    uint64
	NumCacheGetExpires uint64
	NumCacheEvictions  uint64

	NumNvmGets       uint64
	NumNvmGetMiss    uint64
	NumNvmGetExpires uint64
	NumNvmEvictions  uint64

	LoadingCacheLatency PercentileStatsData
	NvmLookupLatency    PercentileStatsData
	NvmInsertLatency    PercentileStatsData
	NvmRemoveLatency    PercentileStatsData
}

// HitRatio return memory cacche hit ratio.
func (s *CacheStats) HitRatio() float64 {
	return float64(s.NumCacheGets-s.NumNvmGets) / float64(s.NumCacheGets)
}

// NvmHitRatio return nvm cacche hit ratio.
func (s *CacheStats) NvmHitRatio() float64 {
	return float64(s.NumNvmGets-s.NumCacheGetMiss) / float64(s.NumNvmGets)
}

type CacheStatsInternal struct {
	counterData    []atomic.Uint64
	percentileData []PercentileStats
	clock          *clock.Clock
}

func NewStats(clock *clock.Clock) *CacheStatsInternal {
	s := &CacheStatsInternal{
		clock:          clock,
		percentileData: make([]PercentileStats, 0),
	}
	s.counterData = make([]atomic.Uint64, counterStatsEnd)
	for i := 0; i < int(percentileStatsEnd-counterStatsEnd-1); i++ {
		s.percentileData = append(s.percentileData, *NewPercentileStats(uint64(clock.NowNano())))
	}
	return s
}

func (s *CacheStatsInternal) Add(t CacheStatsType, value uint64) {
	if s != nil {
		if t < counterStatsEnd {
			s.counterData[int(t)].Add(value)
		} else {
			s.percentileData[int(t-counterStatsEnd)-1].Add(float64(value), uint64(s.clock.NowNano()))
		}
	}
}

func (s *CacheStatsInternal) getCount(t CacheStatsType) uint64 {
	return s.counterData[int(t)].Load()
}

func (s *CacheStatsInternal) getPercentileStatsData(t CacheStatsType) PercentileStatsData {
	return *s.percentileData[int(t-counterStatsEnd)-1].Collect(uint64(s.clock.NowNano()))

}

func (s *CacheStatsInternal) Collect() *CacheStats {
	return &CacheStats{
		NumItems:           s.getCount(NumItems),
		NumCacheGets:       s.getCount(NumCacheGets),
		NumCacheGetMiss:    s.getCount(NumCacheGetMiss),
		NumCacheGetExpires: s.getCount(NumCacheGetExpires),
		NumCacheEvictions:  s.getCount(NumCacheEvictions),

		NumNvmGets:       s.getCount(NumNvmGets),
		NumNvmGetMiss:    s.getCount(NumNvmGetMiss),
		NumNvmGetExpires: s.getCount(NumNvmGetExpires),
		NumNvmEvictions:  s.getCount(NumNvmEvictions),

		LoadingCacheLatency: s.getPercentileStatsData(LoadingCacheLatency),
		NvmLookupLatency:    s.getPercentileStatsData(NvmLookupLatency),
		NvmInsertLatency:    s.getPercentileStatsData(NvmInsertLatency),
		NvmRemoveLatency:    s.getPercentileStatsData(NvmRemoveLatency),
	}
}
