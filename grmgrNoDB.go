//go:build !withdb
// +build !withdb

package grmgr

import (
	"context"
	"fmt"
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

/////////////////////////////////////
//
// register gRoutine start
//
//var StartCh = make(chan Routine, 1)

// count of the number of concurrent instances of a routine. This varies between 0 and ceiling (max concurrency)
type rCntMap map[Routine]int

var rCnt rCntMap

type rWaitMap map[Routine]int

var rWait rWaitMap

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

func (l *Limiter) Unregister() {
	unRegisterCh <- l.r
}

func (l Limiter) RespCh() respCh {
	return l.ch
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

//
//
//

// Note: this package provides a slight enhancement to scaling goroutines the the channel buffer provides.
// It is designed to throttle the number of running instances of a go Routine, i.e. it sets a ceiling on the number of concurrent goRoutines of a particular routine.
// I cannot think of how to get the sync.WaitGroup to provide this feature. It is good for waiting on goRoutines to finish but
// I don't know how to configure sync to set a ceiling on the number of concurrent goRoutines.

// var eventCh chan struct{}{}

//   main
//   	eventCh=make(chan struct{}{},5)
//   	for {
//   		eventCh <- x  // the buffers will fill only if the receiptent of the message does not run a goroutine i.e. is synchronised. if the recipient is not a goroutine their will be only one process
//                        // so to keep the main program from waiting for it to finish we include a buffer on the channel. Hopefully before the buffer fills the recipient will finish and
//                        // execute again.
//   	}                 // if the recipeient runs as go routine then the recipient will empty the buffer as fast as the main will fill it. This may lead to func X spawning a very large
//                        //. number of goroutines the number of which are not impacted by the channel buffer size.
//   }

//   func_ X1
//  	for e = range eventCh { // this will read from channel, start goRoutine and then read from channel again until it is closed
//			go Routine          // The buffer will limit the number of active groutines. As one finishes this will free up a buffer slot and main will fill it with another request to be immediately read by X.
//  	}
//  }
//   func_ X2
//  	for e = range eventCh { // this will read from channel, start goRoutine and then read from channel again until it is closed
//			Routine            // The buffer will limit the number of active groutines. As one finishes this will free up a buffer slot and main will fill it with another request to be immediately read by X.
//  	}
//  }
//
//   So channel buffers are not useful for recipients of channel events that execute go routines. They are useful when the recipient is synchronised with the execution.
//    For goroutine recipients we need a mechanism that can throttle the running of goroutines. This package provides this service.
//
//   func_ Y
//   	z := grmgr.New(<routine>, 5)
//
//   		for e = range eventCh
//   			go Routine          // same as above, unlimited concurrent go routines run. go routine includes Start and End channel messages that increments & decrements internal counter.
//				<-z.Wait()          //  grmgr will send event  on channel if there are less than Ceiling number of concurrent go routines.
//   	}							// Note grmgr limit must be less than channel buffer. So set a large channel buffer and use grmgr to fluctuate between.
//   }

//   func_ Routine {

//	}
//
// New registers a new routine and its ceiling (max concurrency) combination.
func New(r string, c Ceiling, min ...Ceiling) *Limiter {

	m := 1 // minimum ceiling
	if len(min) > 0 {
		m = min[0]
	}
	l, _ := newConfig(r, c, 2, 1, m, "30s")
	return l
}

//limitUnmarshaler := grmgr.NewConfig("unmarshaler", *concurrent*2, 2,1,3,"1m")

func newConfig(r string, c Ceiling, down int, up int, min Ceiling, h string) (*Limiter, error) {

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
	// take a snapshot of rCnt slice every snapInterval seconds - keep upto 2hrs worth of data
	snapInterval = 2
	// save to db every snapReportInterval (seconds)
	snapReportInterval = 10
	// keep live averages at the following reportInterval's (in seconds)
	reportInterval          []int = []int{10, 20, 40, 60, 120, 180, 300, 600, 1200, 2400, 3600, 7200}
	numSamplesAtRepInterval []int

	throttleDownActioned time.Time
	throttleUpActioned   time.Time
)

func init() {
	// prepopulate a useful metric used in calculation of averages
	for _, v := range reportInterval {
		numSamplesAtRepInterval = append(numSamplesAtRepInterval, v/snapInterval)
	}
}

// func scale(c int, perc float64) int {
// 	oc := c
// 	f := float64(c)
// 	f *= perc
// 	c = int(f)
// 	if oc == c {
// 		if perc > 1.0 {
// 			c++
// 		} else {
// 			c--
// 		}
// 	}
// 	return c
// }

// use channels to synchronise access to shared memory ie. the various maps, rLimiterMap.rCntMap.
// "don't communicate by sharing memory, share memory by communicating"
// grmgr runs as a single goroutine with sole access to the shared memory objects. Clients request or update data via channel requests.
// TODO: keep adding entries to map. Determine when to purge entry from maps.
type Config map[string]interface{}

func PowerOn(ctx context.Context, wpStart *sync.WaitGroup, wgEnd *sync.WaitGroup) {

	defer wgEnd.Done()

	var (
		r Routine
		l *Limiter
	)

	rCnt = make(rCntMap)
	rLimit = make(rLimiterMap)
	rWait = make(rWaitMap)

	// throttleDownCh = make(chan struct{})
	// throttleUpCh = make(chan struct{})

	// setup snapshot interrupt goroutine

	var wgStart sync.WaitGroup
	wgStart.Add(1)

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

			}

			rLimit[l.r] = l
			rCnt[l.r] = 0
			rWait[l.r] = 0

		case r = <-EndCh:

			rCnt[r] -= 1
			//logDebug(fmt.Sprintf("EndCh for %s. #concurrent count: %d", r, rCnt[r]))

			if b, ok := rWait[r]; ok {
				if b > 0 && rCnt[r] < rLimit[r].c {
					// Send ack to waiting routine
					rLimit[r].ch <- struct{}{}
					rCnt[r] += 1
					rWait[r] -= 1
				}
			}

		case r = <-rAskCh:

			if rCnt[r] < rLimit[r].c {
				// has ASKed
				rLimit[r].ch <- struct{}{} // proceed to run gr
				rCnt[r] += 1
				logDebug(fmt.Sprintf("has ASKed. Under cnt limit. SEnt ACK on routine channel..for %s  cnt: %d Limit: %d", r, rCnt[r], rLimit[r].c))
			} else {
				logDebug(fmt.Sprintf("has ASKed %s. Cnt [%d] is above limit [%d]. Mark %s as waiting", r, rCnt[r], rLimit[r].c))
				rWait[r] += 1 // log routine as waiting to proceed
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
			delete(rCnt, r)
			delete(rWait, r)
			logAlert(fmt.Sprintf("Unregister %s", r))

		case <-ctx.Done():

			logAlert(fmt.Sprintf("Number of map entries not deleted: %d %d %d ", len(rLimit), len(rCnt), len(rWait)))
			for k, _ := range rLimit {
				logAlert(fmt.Sprintf("rLimit Map Entry: %s", k))
			}
			for k, _ := range rCnt {
				logAlert(fmt.Sprintf("rCnt Map Entry: %s", k))
			}
			for k, _ := range rWait {
				logAlert(fmt.Sprintf("rWait Map Entry: %s", k))
			}
			// TODO: Done should be in a separate select. If a request and Done occur simultaneously then go will randomly pick one.
			// separating them means we have control. Is that the solution. Ideally we should control outside of uuid func().
			logAlert("Shutdown.")
			return

		}

	}
}
