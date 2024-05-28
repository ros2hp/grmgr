# grmgr - goroutine manager

## A quick 101 on CSP?

Built into Go are the foundations of a style of programming called Communicating Sequential Processes (CSP) and in the humble opinion of the author is the languages most identifying and powerful feature. One of the fundamental components of CSP is a **_goroutine_**, which is a function that is executed asynchronously in Go's runtime engine. The other component fundamental to CSP is a **_channel_**, which provides the infrastructure to enable communication between goroutines, i.e. enabling the C in CSP. In the code fragment below the for-loop reads node data from a channel populated by a goroutine (not shown). In the body of the for-loop a function, a **_parallelTask_** is instantiated as a goroutine using the **_go_** keyword. 

```
                var channel = make(chan,node)
	        . . .
		for node := range channel {

			go parallelTask(node)

		}
                . . .
```

As the for-loop body has no constraints there may be an unlimited number of **_parallelTask_** functions started. This may impose a considerable load on the server, depending on how long the task takes to run and how many nodes are queued in the channel, or it may consume all the database connections available in the pool if the task is performing some database operations.  Consequently some control over the number of concurrent **_parallelTask_** needs to be employeed. We refer to this limit as the degree of parallelism of the component.  

To prevent too many concurrent **_parallelTask_** it is relatively easy to introduce some throttling capability. Infact it is a simple as adding a "counter" and create a  **_channel_** to pass back a "finished" message from each **_parallelTask_**.

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

## What are the benefits of using grmgr?

While grmgr can be used to throttle individual goroutines across an application it also provides the facility to dynamically adjust the throttle limits of each goroutine, up or down, in realtime while the application is running. This capability could be used by a server monitor, for example, to send a scale-down message to grmgr in response to a CPU overload alarm. Over time the application would reduce the degree of parallelism of all or some of its components thereby reduce the load on the server. 

Not only could grmgr respond to scaling events from external systems it could also feed into an application dashboard with information about the degree of parallelism in each component of the application in realtime.

So the benefits from grmgr, above normal goroutine throttling are significant, particularly as the coding effort is so minimal.

Lets look at some code examples...

## How to use grmgr.

In the example code segment below we have the same scenario as the previous example but this time it incorporates gmgr to scale a component called "data-propagation" to a maximum of 10 concurrent goroutines.


```

		throttleDP := grmgr.New("data-propagation", 10)
		
		for node := range ch {
	
			throttleDP.Control()                // wait for a response from grmgr to proceed
			
			go processDP(throttleDP, node)

		}
		limiterDP.Wait()
```

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
