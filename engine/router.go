package engine

import (
	"fmt"
	"github.com/funkygao/golib/gofmt"
	"log"
	"sync/atomic"
	"time"
)

type routerStats struct {
	TotalInputMsgN       int64
	PeriodInputMsgN      int32
	TotalInputBytes      int64
	PeriodInputBytes     int64
	TotalProcessedBytes  int64
	TotalProcessedMsgN   int64 // 16 BilionBillion
	PeriodProcessedMsgN  int32
	PeriodProcessedBytes int64
	TotalMaxMsgBytes     int64
	PeriodMaxMsgBytes    int64
}

func (this *routerStats) update(pack *PipelinePack) {
	msgBytes := int64(pack.Message.Size())
	atomic.AddInt64(&this.TotalProcessedBytes, msgBytes)
	atomic.AddInt64(&this.TotalProcessedMsgN, 1)
	atomic.AddInt64(&this.PeriodProcessedBytes, msgBytes)
	atomic.AddInt32(&this.PeriodProcessedMsgN, 1)
	if msgBytes > this.TotalMaxMsgBytes {
		this.TotalMaxMsgBytes = msgBytes
	}
	if msgBytes > this.PeriodMaxMsgBytes {
		this.PeriodMaxMsgBytes = msgBytes
	}

	if pack.Input {
		atomic.AddInt64(&this.TotalInputMsgN, 1)
		atomic.AddInt32(&this.PeriodInputMsgN, 1)
		atomic.AddInt64(&this.TotalInputBytes, msgBytes)
		atomic.AddInt64(&this.PeriodInputBytes, msgBytes)
	}
}

func (this *routerStats) resetPeriodCounters() {
	this.PeriodProcessedBytes = int64(0)
	this.PeriodInputBytes = int64(0)
	this.PeriodInputMsgN = int32(0)
	this.PeriodProcessedMsgN = int32(0)
	this.PeriodMaxMsgBytes = int64(0)
}

func (this *routerStats) render(logger *log.Logger, elapsed int) {
	logger.Printf("Total:%10s %10s speed:%6s/s %10s/s max: %s/%s",
		gofmt.Comma(this.TotalProcessedMsgN),
		gofmt.ByteSize(this.TotalProcessedBytes),
		gofmt.Comma(int64(this.PeriodProcessedMsgN/int32(elapsed))),
		gofmt.ByteSize(this.PeriodProcessedBytes/int64(elapsed)),
		gofmt.ByteSize(this.PeriodMaxMsgBytes),
		gofmt.ByteSize(this.TotalMaxMsgBytes))
	logger.Printf("Input:%10s %10s speed:%6s/s %10s/s",
		gofmt.Comma(int64(this.PeriodInputMsgN)),
		gofmt.ByteSize(this.PeriodInputBytes),
		gofmt.Comma(int64(this.PeriodInputMsgN/int32(elapsed))),
		gofmt.ByteSize(this.PeriodInputBytes/int64(elapsed)))
}

type messageRouter struct {
	hub chan *PipelinePack

	stats routerStats

	removeFilterMatcher chan *Matcher
	removeOutputMatcher chan *Matcher

	filterMatchers []*Matcher
	outputMatchers []*Matcher
}

func NewMessageRouter() (this *messageRouter) {
	this = new(messageRouter)
	this.hub = make(chan *PipelinePack, Globals().PluginChanSize)
	this.stats = routerStats{}
	this.removeFilterMatcher = make(chan *Matcher)
	this.removeOutputMatcher = make(chan *Matcher)
	this.filterMatchers = make([]*Matcher, 0, 10)
	this.outputMatchers = make([]*Matcher, 0, 10)

	return
}

func (this *messageRouter) addFilterMatcher(matcher *Matcher) {
	this.filterMatchers = append(this.filterMatchers, matcher)
}

func (this *messageRouter) addOutputMatcher(matcher *Matcher) {
	this.outputMatchers = append(this.outputMatchers, matcher)
}

func (this *messageRouter) reportMatcherQueues(logger *log.Logger) {
	globals := Globals()
	s := fmt.Sprintf("Queued hub=%d", len(this.hub))
	if len(this.hub) == globals.PluginChanSize {
		s = fmt.Sprintf("%s(F)", s)
	}

	for _, m := range this.filterMatchers {
		s = fmt.Sprintf("%s %s:%d", s, m.runner.Name(), len(m.InChan()))
		if len(m.InChan()) == globals.PluginChanSize {
			s = fmt.Sprintf("%s(F)", s)
		}
	}
	for _, m := range this.outputMatchers {
		s = fmt.Sprintf("%s %s:%d", s, m.runner.Name(), len(m.InChan()))
		if len(m.InChan()) == globals.PluginChanSize {
			s = fmt.Sprintf("%s(F)", s)
		}
	}

	logger.Println(s)
}

// Dispatch pack from Input to MatchRunners
func (this *messageRouter) Start() {
	var (
		globals    = Globals()
		ok         = true
		pack       *PipelinePack
		ticker     *time.Ticker
		matcher    *Matcher
		foundMatch bool
	)

	ticker = time.NewTicker(time.Second * time.Duration(globals.TickerLength))
	defer ticker.Stop()

	if globals.Verbose {
		globals.Printf("Router started with ticker=%ds\n", globals.TickerLength)
	}

	if globals.Verbose {
		go func() {
			t := time.NewTicker(time.Second * time.Duration(globals.TickerLength))
			defer t.Stop()

			for _ = range t.C {
				this.reportMatcherQueues(globals.Logger)
			}
		}()
	}

LOOP:
	for ok {
		select {
		case matcher = <-this.removeOutputMatcher:
			this.removeMatcher(matcher, this.outputMatchers)

		case matcher = <-this.removeFilterMatcher:
			this.removeMatcher(matcher, this.filterMatchers)

		case <-ticker.C:
			this.stats.render(globals.Logger, globals.TickerLength)
			this.stats.resetPeriodCounters()

		case pack, ok = <-this.hub:
			if !ok {
				globals.Stopping = true
				break LOOP
			}

			this.stats.update(pack)

			pack.diagnostics.Reset()
			foundMatch = false

			// If we send pack to filterMatchers and then outputMatchers
			// because filter may change pack Ident, and this pack bacuase
			// of shared mem, may match both filterMatcher and outputMatcher
			// then dup dispatching happens!!!
			//
			// We have to dispatch to Output then Filter to avoid that case
			for _, matcher = range this.outputMatchers {
				// a pack can match several Output
				if matcher != nil && matcher.match(pack) {
					foundMatch = true

					pack.IncRef()
					pack.diagnostics.AddStamp(matcher.runner)
					matcher.InChan() <- pack
				}
			}

			// got pack from Input, now dispatch
			// for each target, pack will inc ref count
			// and the router will dec ref count only once
			for _, matcher = range this.filterMatchers {
				// a pack can match several Filter
				if matcher != nil && matcher.match(pack) {
					foundMatch = true

					pack.IncRef()
					pack.diagnostics.AddStamp(matcher.runner)
					matcher.InChan() <- pack
				}
			}

			if !foundMatch {
				// Maybe we closed all filter/output inChan, but there
				// still exits some remnant packs in router.hub
				globals.Printf("Found no match: " + pack.String())
			}

			// never forget this!
			pack.Recycle()
		}
	}

	if globals.Verbose {
		globals.Println("Router shutdown.")
	}
}

func (this *messageRouter) removeMatcher(matcher *Matcher, matchers []*Matcher) {
	globals := Globals()
	for idx, m := range matchers {
		if m == matcher {
			if globals.Verbose {
				globals.Printf("Closed matcher for %s", m.runner.Name())
			}

			// in golang, close means we can no longer send to that chan
			// but consumers can still recv from the chan
			close(m.InChan())
			matchers[idx] = nil
			return
		}
	}
}
