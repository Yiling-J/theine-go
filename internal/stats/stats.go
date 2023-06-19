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
			s.percentileData[int(t-counterStatsEnd)].Add(float64(value), uint64(s.clock.NowNano()))
		}
	}
}
