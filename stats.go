package dao

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/teachme2/gorm-dao/utils"
)

const (
	maxStatsSnapshots = 10
	statsIntervalMins = 10
)

type dbQueryStat struct {
	query         string
	count         int
	min, max, sum time.Duration
	avg           time.Duration
}

type dbQueryStats []dbQueryStat

func (s dbQueryStats) Len() int           { return len(s) }
func (s dbQueryStats) Less(i, j int) bool { return s[i].avg < s[j].avg }
func (s dbQueryStats) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type dbSnapshot struct {
	query string
	d     time.Duration
}

type stdoutDbStatsCollector struct {
	stats     map[string]dbQueryStat
	statsLock sync.RWMutex
	statsCh   chan dbSnapshot
}

func newStdoutStatsCollector() DbStatsCollector {
	s := new(stdoutDbStatsCollector)
	s.statsCh = make(chan dbSnapshot)
	s.stats = map[string]dbQueryStat{}

	go func() {
		defer func() {
			_ = utils.CheckPanic(recover())
		}()
		for {
			s.PrintDbStats(30)
			time.Sleep(statsIntervalMins * time.Minute)
			//time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		for st := range s.statsCh {
			if len(s.stats) > 500 {
				fmt.Fprintln(os.Stderr, "too many log queries", len(s.stats))
			}
			if len(s.stats) > 1000 {
				fmt.Fprintln(os.Stderr, "too many log queries", len(s.stats), "=> cleaning")
				s.PrintDbStats(500)
				s.stats = map[string]dbQueryStat{}
			}

			s.statsLock.RLock()
			curr := s.stats[st.query]
			s.statsLock.RUnlock()

			curr.count++
			curr.query = st.query
			curr.sum = curr.sum + st.d
			curr.avg = time.Duration(int64(curr.sum) / int64(curr.count))
			if curr.max == 0 || st.d > curr.max {
				curr.max = st.d
			}
			if curr.min == 0 || st.d < curr.min {
				curr.min = st.d
			}

			s.statsLock.Lock()
			s.stats[st.query] = curr
			s.statsLock.Unlock()
		}
	}()

	return s
}

func (s *stdoutDbStatsCollector) AddDbStats(c context.Context, since time.Time, queryFmt string, params ...interface{}) {
	if queryFmt == "" {
		fmt.Fprintf(os.Stderr, "empty query descriptor:"+string(debug.Stack()))
	}
	duration := time.Since(since)
	query := fmt.Sprintf(queryFmt, params...)
	s.statsCh <- dbSnapshot{query: query, d: duration}
}

func (s *stdoutDbStatsCollector) PrintDbStats(count int) {
	s.statsLock.RLock()
	defer s.statsLock.RUnlock()

	var list []dbQueryStat
	for _, s := range s.stats {
		list = append(list, s)
	}
	var byts bytes.Buffer
	byts.WriteString("SLOWEST QUERIES:\n")
	sort.Sort(dbQueryStats(list))
	for i := len(list) - 1; i >= 0 && i > len(list)-count; i-- {
		byts.WriteString(fmt.Sprintf("%s\n\tavg/min/max %s/%s/%s, n=%d\n", list[i].query, list[i].avg.String(), list[i].min.String(), list[i].max.String(), list[i].count))
	}
	fmt.Println(byts.String())
}

type DummyDbStatsCollector struct{}

func (DummyDbStatsCollector) AddDbStats(c context.Context, since time.Time, query string, params ...interface{}) {
}
