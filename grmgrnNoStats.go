//go:build !withstats
// +build !withstats

package grmgr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// statistics monitor
	statsSystemTag string = "__grmgr"
)

type Routine = string

type Ceiling = int

type throttle_ byte

func (t throttle_) Up() {
	throttleUpCh <- Routine("__all")
}

func (t throttle_) Down() {
	throttleDownCh <- Routine("__all")
}

func (t throttle_) Stop() {}

func (t throttle_) String() {}

var Control throttle_

// Channels
var (
	EndCh          = make(chan Routine, 1)
	throttleDownCh = make(chan Routine)
	throttleUpCh   = make(chan Routine)
	//
	rAskCh     = make(chan Routine)
	rExpirehCh = make(chan Routine)
	//
)

// Limiter
type respCh chan struct{}

type Limiter struct {
	r  Routine // modified routine to make unique
	or Routine // original routine
	//
	c    Ceiling // ceiling value (starts at oc value)
	maxc Ceiling // original (maximum) ceiling
	minc Ceiling // minimum ceiling
	//
	up   int // scale up by value
	down int // scale down by value (down <= up)
	//
	hold time.Duration // hold at current ceiling for duration
	//
	ch respCh
	on bool // send Wait response
	//
	wg    sync.WaitGroup
	rCnt  int // replace rCnt
	rWait int // replace rWait
}

func (l *Limiter) Ask() {
	rAskCh <- l.r
}

// func (l *Limiter) StartR() {
// 	//	StartCh <- l.r
// }

func (l *Limiter) EndR() {
	EndCh <- l.r
}

func (l *Limiter) Done() {
	EndCh <- l.r
}

func (l *Limiter) Unregister() {
	unRegisterCh <- l.r
}

func (l *Limiter) Delete() {
	unRegisterCh <- l.r
}

func (l Limiter) RespCh() respCh {
	return l.ch
}

func (l Limiter) Control() {
	rAskCh <- l.r
	<-l.ch
}

// Wait for all groutine to finish i.e rCnt[l.r] == 0
func (l *Limiter) Wait() {
	l.wg.Wait()
}

func (l Limiter) Valve() {
	l.Control()
}

func (l Limiter) Routine() Routine {
	return l.r
}

func (l Limiter) Up() {
	throttleUpCh <- l.r
}

func (l Limiter) Down() {
	throttleDownCh <- l.r
}

type rLimiterMap map[Routine]*Limiter

var (
	rLimit       rLimiterMap
	registerCh   = make(chan *Limiter)
	unRegisterCh = make(chan Routine)
)

// New creates a new Limiter  and sets a ceiling
func New(r string, c Ceiling, min ...Ceiling) *Limiter {

	m := 1 // minimum ceiling
	if len(min) > 0 {
		m = min[0]
	}
	l, _ := NewConfig(r, c, 2, 1, m, "30s")
	return l
}

// NewConfig creates a limiiter with extra configuration associated with throttling.
func NewConfig(r string, c Ceiling, down int, up int, min Ceiling, h string) (*Limiter, error) {

	hold, err := time.ParseDuration(h)
	if err != nil {
		panic(err)
		return nil, err
	}

	l := Limiter{c: c, maxc: c, minc: min, up: up, down: down, r: Routine(r), or: Routine(r), ch: make(chan struct{}), on: true, hold: hold}
	registerCh <- &l
	logAlert(fmt.Sprintf("New Routine %q  Ceiling: %d [min: %d, down: %d, up: %d, hold: %s]", r, c, min, down, up, h))
	return &l, nil
}

var (
	// keep live averages at the following reportInterval's (in seconds)
	throttleDownActioned time.Time
	throttleUpActioned   time.Time
)

// TODO: keep adding entries to map. Determine when to purge entry from maps.
type Config map[string]interface{}

func PowerOn(ctx context.Context, wpStart *sync.WaitGroup, wgEnd *sync.WaitGroup, cfg ...Config) {

	defer wgEnd.Done()

	var (
		r Routine
		l *Limiter
		//snapshot reporting
		s, rsnap int
		ok       bool
		dbname   string
		reportOn bool
		runId    uuid.UID
		reptbl   string
	)

	if len(cfg) > 0 {
		cfg := cfg[0]
		for k, v := range cfg {
			switch strings.ToLower(k) {
			case "runid":
				reportOn = true
				runId, ok = v.(uuid.UID)
				if !ok {
					logErr(fmt.Errorf("runid should be a tx.uuid.UID"))
				}
			case "dbname":
				reportOn = true
				dbname = v.(string)
			case "table":
				reptbl = v.(string)
			default:
				logErr(fmt.Errorf("not a supported config key  %q", k))
			}
		}
		if len(runId) == 0 {
			logErr(fmt.Errorf("must supply a runid of uuid.UID type"))
		}
		if len(dbname) == 0 {
			logAlert(`no database name specified in config. Will use "default"`)
			dbname = "default"
		}
		if len(reptbl) == 0 {
			logAlert(`no database name specified in config. Will use "runStats"`)
			reptbl = "runStats"
		}
	}

	rLimit = make(rLimiterMap)

	// throttleDownCh = make(chan struct{})
	// throttleUpCh = make(chan struct{})

	ctxSnap, cancelSnap := context.WithCancel(context.Background())
	var wgSnap sync.WaitGroup

	logAlert("Started.")
	wpStart.Done()

	for {

		select {

		case l = <-registerCh:

			// change the ceiling by passing in Limiter struct. As struct is a non-ref type, l is a copy of struct passed into channel. Ref typs, spcmf - slice, pointer, map, func, channel
			// check not already registered -
			// generate unique label
			var e byte = 65
			for {
				if _, ok := rLimit[l.r]; !ok {
					// unique label
					break
				}
				// routine r already exists, generate a unique value
				l.r += string(e)
				e++
				if e > 175 {
					// generate a UUID instead
					uid, _ := uuid.MakeUID()
					l.r = uid.String()
				}
			}

			rLimit[l.r] = l

		case r = <-EndCh:

			if l, ok := rLimit[r]; ok {
				l.wg.Done()
				l.rCnt--

				if l.rWait > 0 && l.rCnt < l.c {
					// Send ack to waiting routine
					l.ch <- struct{}{}
					l.rCnt++
					l.rWait--
				}

			} else {
				panic(fmt.Errorf("expected limiter %s in rLimit got nil", r))
			}

		case r = <-rAskCh:

			if l, ok := rLimit[r]; ok {
				l.wg.Add(1)

				if l.rCnt < l.c {
					// has ASKed
					l.ch <- struct{}{} // proceed to run gr
					l.rCnt++
					logDebug(fmt.Sprintf("has ASKed. Under cnt limit. SEnt ACK on routine channel..for %s  cnt: %d Limit: %d", r, l.rCnt, l.c))
				} else {
					logDebug(fmt.Sprintf("has ASKed %s. Cnt [%d] is above limit [%d]. Mark %s as waiting", r, l.rCnt, l.c))
					l.rWait++ // log routine as waiting to proceed
				}

			} else {
				panic(fmt.Errorf("expected limiter %s in rLimit got nil", r))
			}

		case r = <-throttleDownCh:

			allr := make(rLimiterMap)
			if r == Routine("__all") {
				allr = rLimit
			} else {
				allr[r] = rLimit[r]
			}
			t0 := time.Now()

			for _, v := range rLimit {
				//

				if t0.Sub(throttleDownActioned) < v.hold {
					logAlert("throttleDown: to soon to throttle down after last throttled action")
				} else {

					if t0.Sub(throttleUpActioned) < v.hold {
						logAlert("throttleDown: to soon to throttle down after last throttled action")
					} else {

						// throttle down by 20%. Once changed cannot be modified for 2 minutes.

						v.c -= v.down

						if v.c < v.minc {
							v.c = v.minc
							logAlert(fmt.Sprintf("throttleDown: Throttling has reached minimum allowed [%d], for %s", v.c, v.minc))
						} else {
							logAlert(fmt.Sprintf("throttleDown: %s throttled down to %d [minimum: %d]", v.or, v.c, v.minc))
						}
					}
				}
			}
			throttleDownActioned = t0

		case r = <-throttleUpCh:

			allr := make(rLimiterMap)
			if r == Routine("__all") {
				allr = rLimit
			} else {
				allr[r] = rLimit[r]
			}
			t0 := time.Now()
			for _, v := range rLimit {
				//

				if t0.Sub(throttleDownActioned) < v.hold {
					logAlert("throttleDown: to soon to throttle down after last throttled action")
				} else {

					if t0.Sub(throttleUpActioned) < v.hold {
						logAlert("throttleDown: to soon to throttle down after last throttled action")
					} else {

						// throttle down by 20%. Once changed cannot be modified for 2 minutes.

						v.c += v.up

						if v.c > v.maxc {
							v.c = v.maxc
							logAlert(fmt.Sprintf("throttleDown: Throttling has reached minimum allowed [%d], for %s", v.c, v.minc))
						} else {
							logAlert(fmt.Sprintf("throttleDown: %s throttled down to %d [minimum: %d]", v.or, v.c, v.minc))
						}
					}
				}
			}
			throttleUpActioned = t0

		case r = <-unRegisterCh:

			delete(rLimit, r)
			delete(csnap, r)
			logAlert(fmt.Sprintf("Unregister %s", r))

		case <-ctx.Done():
			cancelSnap()
			logAlert("Waiting for internal snap service to shutdown...")
			wgSnap.Wait()
			logAlert("Internal snap service shutdown")
			logAlert(fmt.Sprintf("Number of map entries not deleted: %d ", len(rLimit)))
			for k, _ := range rLimit {
				logAlert(fmt.Sprintf("rLimit Map Entry: %s", k))
			}
			// // TODO: Done should be in a separate select. If a request and Done occur simultaneously then go will randomly pick one.
			// separating them means we have control. Is that the solution. Ideally we should control outside of uuid func().
			logAlert("Shutdown.")
			return
		}
	}
}
