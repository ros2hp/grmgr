# grmgr - goroutine manager

## A quick 101 on CSP?

Built into Go are the foundations of a style of programming called Communicating Sequential Processes (CSP) and in my humble opinion is the languages most identifying and powerful feature. One of the fundamental components of CSP is Go's **_goroutine_**, which enable functions to be executed asynchronously using Go's built in runtime engine. The other component fundamental to CSP is Go's **_channel_**, which provides the infrastructure to enable communication between goroutines, i.e. the C in  CSP. In the two line code fragment we are reading data from a channel, which has been populated by a goroutine (not shown) in a for-loop and instantiating a parraleTask function asynchronously using the **_go_** keyword. 

```
	        . . .
		for node := range channel {
			go parallelTask(node)
		}
                . . .
```

This examples will instantiate an unlimited number of asyncrhonous functions , which, may or may not, present a resourcing issue to the server. If the parallelTask is not particularly compute instensive (i.e. performs some IO operations) and the server has prodigous resources, most notably cores, then the threshold at which it can comfortably execute concurrent paralelTask's will be relatively high.

To prevent too many concurrent tasks from overwhelming the server it is relatively easy to implement some control. Just introduce a "counter" and create a  **_channel_** to pass back a "finished" message from the parallelTask.

```
	        . . .
                const MAX_CONCURRENT = 100
                var channel = make(chan,message)
                let task_counter = 0

		for node := range channel {

			go parallelTask(node, channel)
                        task_counter++

                        if task_counter == MAX_CONCURRENT {
                             <-channel                       // block here and wait on any one of the concurrent parallelTask to finish
                             task_counter--
                        }
		}
                . . .
```
So, if it is so easy to manage the number of concurrent goroutines, why the need for a package that claims to manager goroutines?

## What does grmgr do?

grmgr has a single view of all the goroutines running under its management. grmgr is typically used in large and complex programs that have many tens of concurrently running components each of which may spawn a large number parallel tasks. grmgr relieves the programmer of having to determining, at runtime, how to adjust the number of parallel goroutines the component can instantiate. This might be in response to some "monitor" component that has determined the server spare capacity to run more goroutines or it is under too much load and the program needs to reduce the number of concurrent goroutines across one or more of its components. It should be stated that the monitor component is not currently part of grmgr but it may be a worthwhile development if there is enough interest in it.

There are a number of ways grmgr can be implemented of course, but I chose to implement the solution as a "service", which runs as goroutine. A service simplifies the concurrency issues managing access to a single respository containing the state of each component that uses grmgr. It also means that a goroutine constraint, such as the maximum number of parallel goroutines that can run for a component, can be safely and easily modified (up or down) at runtime, by sending a "modify" message to the grmgr service. This  begs the question "what determines a safe number of parallel tasks to run for a component?" This part of the grmgr service has yet to be developed but the monitor component will require access to something like an grmgr service to implement its decisions. As a minimum the monitor could regulate the load on the server (CPU, memory etc) by messaging grmgr to either increase or decresase the number of concurrent goroutines. grmgr would use its knowledge from the repository of performance metrics for each goroutine task under its management (such as duration of the task) to determine how best to implement the request from the "monitor".

A further development would use grmgr to drive a dashboard showing the program components and the number of goroutines running in each in near realtime. grmgr already contains a small repository of history data containing for each component the number of concurrent goroutines it has spawned in the last 30 second, 1minute, 5minuter..... 

So what we have in the grmgr currently is the minimum of functionality. It simply controls the number of concurrent goroutines a component can instantiate and provides a rudimentary repository of its history.

## How to use grmgr.

In the example below we have a typical scenario where a stream of potentially thousands of nodes is read from a channel. Each node is passed to a goroutine, __processDP()__ for processing. How this would use grmgr
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
