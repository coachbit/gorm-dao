package stats

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	utils "github.com/coachbit/gorm-dao/dao/daoutils"
)

type queryStat struct {
	queryDescriptor string
	count           int
	min, max, sum   time.Duration
	avg             time.Duration
}

type queryStats []queryStat

func (s queryStats) Len() int           { return len(s) }
func (s queryStats) Less(i, j int) bool { return s[i].avg < s[j].avg }
func (s queryStats) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type dbSnapshot struct {
	query string
	d     time.Duration
}

type StatsCollector struct {
	title      string
	top        int
	stats      map[string]queryStat
	statsLock  sync.RWMutex
	statsCh    chan dbSnapshot
	LastReport string
}

func NewStatsCollector(title string, interval time.Duration, top int) *StatsCollector {
	s := new(StatsCollector)
	s.top = top
	s.statsCh = make(chan dbSnapshot)
	s.stats = map[string]queryStat{}
	s.title = strings.ToUpper(title)

	go func() {
		defer func() { _ = utils.CheckPanicOrLog(recover(), nil) }()

		// Initially a few shorter periods
		time.Sleep(10 * time.Minute)
		s.PrintStatsAndReset()

		time.Sleep(30 * time.Minute)
		s.PrintStatsAndReset()

		for {
			time.Sleep(interval)
			s.PrintStatsAndReset()
			//time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		for st := range s.statsCh {
			if len(s.stats) > 500 {
				fmt.Fprintln(os.Stderr, "too many queries", len(s.stats))
			}
			if len(s.stats) > 1000 {
				fmt.Fprintln(os.Stderr, "too many queries", len(s.stats), "=> cleaning")
				s.PrintStatsAndReset()
				s.stats = map[string]queryStat{}
			}

			s.statsLock.RLock()
			curr := s.stats[st.query]
			s.statsLock.RUnlock()

			curr.count++
			curr.queryDescriptor = st.query
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

func (s *StatsCollector) Reset() {
	s.statsLock.Lock()
	defer s.statsLock.Unlock()
	s.stats = map[string]queryStat{}
}

func (s *StatsCollector) AddStats(c context.Context, since time.Time, queryFmt string, params ...interface{}) {
	if queryFmt == "" {
		fmt.Fprintf(os.Stderr, "empty query descriptor:"+string(debug.Stack()))
	}
	duration := time.Since(since)
	query := fmt.Sprintf(queryFmt, params...)
	s.statsCh <- dbSnapshot{query: query, d: duration}
}

func (s *StatsCollector) formatDuration(d time.Duration) string {
	return utils.FormatTime(d, time.Millisecond, 3)
}

func (s *StatsCollector) PrintStatsAndReset() {
	var list []queryStat

	s.statsLock.RLock()
	for _, s := range s.stats {
		list = append(list, s)
	}
	s.statsLock.RUnlock()

	str := utils.NewStringBuilder()

	str.Appendln(s.title, " (avg/min/max/count):")
	if len(list) == 0 {
		str.Appendln("empty")
		goto report
	}

	sort.Sort(queryStats(list))
	for count, i := 0, len(list)-1; i >= 0; i-- {
		str.Appendf("%50s  %s\n", fmt.Sprint(s.formatDuration(list[i].avg), "/", s.formatDuration(list[i].min), "/", s.formatDuration(list[i].max), "/n=", list[i].count), list[i].queryDescriptor)
		count++
		if count > s.top {
			goto report
		}
	}

report:
	s.LastReport = str.String()
	fmt.Println(s.LastReport)
	s.Reset()
}
