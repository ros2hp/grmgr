//go:build withdb
// +build withdb

package grmgr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ros2hp/method-db/mut"
	"github.com/ros2hp/method-db/tbl"
	"github.com/ros2hp/method-db/tx"
	"github.com/ros2hp/method-db/uuid"
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
	l, _ := NewConfig(r, c, 2, 1, m, "30s")
	return l
}

//limitUnmarshaler := grmgr.NewConfig("unmarshaler", *concurrent*2, 2,1,3,"1m")

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

	rCnt = make(rCntMap)
	rLimit = make(rLimiterMap)
	rWait = make(rWaitMap)
	csnap := make(map[string][]int)  //cumlative snapshots
	csnap_ := make(map[string][]int) //shadow copy of csnap used by reporting system

	// throttleDownCh = make(chan struct{})
	// throttleUpCh = make(chan struct{})

	// setup snapshot interrupt goroutine
	snapCh := make(chan time.Time)

	ctxSnap, cancelSnap := context.WithCancel(context.Background())
	var wgSnap sync.WaitGroup
	var wgStart sync.WaitGroup
	wgStart.Add(1)
	wgSnap.Add(1)

	// start report-snapshot goroutine
	if reportOn {
		go func() {
			wgStart.Done()
			defer wgSnap.Done()
			// wait for grmgr to start for loop
			wpStart.Wait()
			logAlert("Report-snapshot Powering up...")
			for {
				select {
				case t := <-time.After(time.Duration(snapInterval) * time.Second):
					snapCh <- t
				case <-ctxSnap.Done():
					logAlert("Report-snapshot Shutdown.")
					return
				}
			}

		}()
		logAlert("Waiting for gr monitor to power up...")
		// wait for snap interrupter to start
		wgStart.Wait()
	}
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

		case <-snapCh:

			s++
			rsnap += snapInterval
			//rsnap += snapInterval
			// cumulate rCnt(one result per gr) into csnap (history)
			for k, v := range rCnt {
				csnap[k] = append(csnap[k], v)
			}
			// save to db every snapReport seconds (default: 20s)
			if rsnap == snapReportInterval {

				// update shadow copy of csnap (csnap_) with latest results generated since last snap Report
				// csnap_ is passed to reporting system to be read while csnap is being updated by time.After() - hence copy.
				for k, v := range csnap {
					if len(v) < s {
						// not enough snapshots taken for limiter k - ignore for this report
						continue
					}
					for _, vv := range v[len(v)-s:] {
						csnap_[k] = append(csnap_[k], vv)
					}
				}
				report(dbname, reptbl, csnap_, runId, snapInterval, snapReportInterval)
				logDebug("gr dump report to table completed...")
				rsnap, s = 0, 0

			}

		case r = <-unRegisterCh:

			delete(rLimit, r)
			delete(rCnt, r)
			delete(rWait, r)
			delete(csnap, r)
			logAlert(fmt.Sprintf("Unregister %s", r))

		case <-ctx.Done():
			cancelSnap()
			logAlert("Waiting for internal snap service to shutdown...")
			wgSnap.Wait()
			logAlert("Internal snap service shutdown")
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
		}
	}
}

func report(dbname string, reptbl string, snap map[string][]int, runid uuid.UID, snapInterval, snapReportInterval int) {

	// report average cnt for each interval for each grmgr limiter (throttler)
	reportAvg := make(map[string]map[int]float64, len(snap))
	// number of samples in a reporting interval (e.g. 10/2=5)
	ns := numSamplesAtRepInterval

	// populate reportAvg with map entries containing keys of sample size for each interval e.g. 10:5, 20:10, 40:20 for snapInterval of 2
	for k, _ := range snap {

		sample := make(map[int]float64, len(reportInterval))

		for _, v := range reportInterval {
			i := v / snapInterval
			sample[i] = float64(0)

		}
		reportAvg[k] = sample

	}
	for k, v := range snap {

		ii, sum := 0, 0
		// latest to oldest snapshot.
		// terminate all entries after 2hrs =(2*3600)/snapInterval = 3600
		for i := len(v); i > 0; i-- {

			ii++
			sum += v[i-1]

			for kk, _ := range reportAvg[k] {

				if kk == ii {
					// calc average
					reportAvg[k][kk] = float64(sum) / float64(kk)
					break
				}

			}
			// drop expired entries ie. > 2hrs
			if ii == ns[len(ns)-1] {
				logDebug("drop expired snap entries..")
				snap[k] = v[1:]
			}

		}
	}
	logDebug("About to dump report to table...")
	// table columns in mon_gr containing averages
	col := []string{"s10", "s20", "s40", "m1", "m2", "m3", "m5", "m10", "m20", "m40", "h1", "h2"}
	// update database - this should be a merge opeation based on what key?

	for k, v := range reportAvg {

		mtx := tx.New(statsSystemTag).DB(dbname)

		m := mtx.NewMerge(tbl.Name(reptbl)).AddMember("run", runid, mut.IsKey).AddMember("sortk", "gr#"+k, mut.IsKey)
		for i, c := range col {
			m.AddMember(c, v[ns[i]])
		}
		err := mtx.Execute()
		if err != nil {
			logErr(err)
		}
	}

	//	}

}
