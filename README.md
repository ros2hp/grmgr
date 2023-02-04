# grmgr
Wherever there is an opportunity to instantiate a large number of goroutines, typically in a for-loop, **_grmgr_** (GoRoutine ManaGeR) can be used to constrain the number of concurrent goroutines to a fixed number until the loop is exited. 

In the example below we have a typical scenario where a stream of potentially thousands of nodes is read from a channel. Each node is passed to a goroutine, __processDP()__.
```

		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
	
			limiterDP.Control()  // may block
			
			go processDP(limiterDP, node)
		}
		limiterDP.Wait()
```
Rather than instantiate thousands of concurrent goroutintes the developer creates a **_grmgr_** Limiter, specifying a ceiling value, and use the Control() method in the for-loop to constrain the number of concurrent goroutines instantiated.

The Control() method will block when the number of concurrent groutines exceeds the ceiling value of the Limiter. When one of the goroutines finish Control() will be immedidately unblocked.
In this way **_grmgr_** can maintain a fixed number of concurrent goroutines, equal to the ceiling value. The Limiter also comes with a Wait(), which emulates sync.Wait(), and in this case will wait until the last of the goroutines finish.

To safeguards the state of each Limiter, **_grmgr_** runs as a service, meaning  **_grmgr_** runs as a goroutine and communicates only via channels, which is encapsulated in each of the __Limiter__ methods. In this way access to shared data is serialised and **_grmgr_** is made concurrency safe.

So before using  **_grmgr_**  you must start the service using:

```
 	go grmgr.PowerOn(ctx, &wpStart, &wpEnd) 
```

Where wpStart and wpEnd are instances of sync.WaitGroup used to synchronise when the service is started and shutdown via the cancel context ctx.

The contents of method Control() illustrates the communication with the **_grmgr_** service. Please review the code if you want to understand more of the details.

```
	func (l Limiter) Control() {
		rAskCh <- l.r
		<-l.ch
	}
```
 
When a Limiter is nolonger needed it should be deleted:

```
	limiterDP.Delete()
```

A goroutine that is under the control of **_grmgr_** accepts a Limiter as an argument. Execute the __Done()__ method when the goroutine is finished.

```
	defer limiterDP.Done()
```

To configure a logger for __grmgr__ use the following:
```
	grmgr.SetLogger(<logger>, <log level>) 
```
Available log levels are:
```
	const (
		Alert LogLvl = iota
		Debug
		NoLog
	)
```

 **_grmgr_** comes in two editions, one which captures runtime metadata to a database in near realtime (build tag "withstats") and one without metadata reporting (no tag).
 **_grmgr_** without reporting is recommended for all use cases outside of grmgr development.

The following code snippet illustrates a complete end-to-end use of the **_grmgr_** service.

```
		// create a context
		ctx, cancel := context.WithCancel(context.Background())
		
		// optional: send a logger and log level to grmgr 
		grmgr.SetLogger(logr, grmgr.Alert)
		
		// start grmgr service
		go grmgr.PowerOn(ctx, wpStart, wpEnd) 
		
		// create a limiter for DP process setting the ceiling to 10.
		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
			
			// may block and wait for number of concurrent instances of goroutine 'processDP' to drop below ceiling.
			limiterDP.Control()
			
			go processDP(limiterDP, node)
		}
		// wait for the last of the DP goroutines to finish
		limiterDP.Wait()
		
		// free up limiter
		limiterDP.Delete()
		
		. . .
		
		// shudown grmgr service (and all other services that use the context)
		cancel()
		
		
```

## Limiter Throttle

To configure a non-default throttle for a Limiter use NewConfig()
```
// NewConfig - configure a Limiter with non-default throttle settings
// r : limiter name
// c : ceiling value (also the maximum value for the throttle)
// down : adjust current ceiling down by specified value
// up :   adjust current ceiling up by specified value
// min : minimum value for ceiling
// h : hold any change for this duration (in a string value that can be converted to time.Duration) e.g. "5s" for five seconds

func NewConfig(r string, c Ceiling, down int, up int, min Ceiling, h string) (*Limiter, error) {

```
The New() constructor for Limiter will create a default throttle.
```
// New() - configure a Limiter with default throttle settings
// r : limiter name
// c : ceiling 
// min : minimum value for ceiling [default 1]
func New(r string, c Ceiling, min ...Ceiling) *Limiter {

	 NewConfig(r, c, 2, 1, m, "30s")
```
The associated throttle methods for a Limiter, l:
```
	l.Up()

	l.Down()
```
