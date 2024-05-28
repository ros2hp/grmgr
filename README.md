# grmgr - goroutine manager

## A quick 101 on CSP?

Built into Go are the foundations of a style of programming called Communicating Sequential Processes (CSP) and in my humble opinion is the languages most identifying and powerful feature. One of the fundamental components of CSP is Go's **_goroutine_**, which enable functions to be executed asynchronously using Go's built in runtime engine. The other component fundamental to CSP is Go's **_channel_**, which provides the infrastructure to enable communication between goroutines, i.e. enabling the C in  CSP. In the two line code fragment we are reading data from a channel, which has been populated by a goroutine (not shown) in a for-loop and instantiating a parraleTask function asynchronously using the **_go_** keyword. 

```
	        . . .
		for node := range channel {
			go parallelTask(node)
		}
                . . .
```

This examples will instantiate an unlimited number of asyncrhonous functions, which may present a resourcing issue to the server. If the **_parallelTask_** is not particularly compute instensive (i.e. performs some blocking operations like IO or database requests) and the server has prodigous resources, most notably cores, then the threshold at which it can comfortably execute multiple concurrent **_parallelTask_** will be relatively high. Of course if **_parallelTask_** is communicating with a database then it will eventually reach the maximum number of connections a database can support or that have been configured into the connection pool. Consequently some control is required over the number of concurrent asyncrhonous functions that are executing. 

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

So the benefits from grmgr, above normal goroutine throttling are significant, particularly as the coding effort is minimal.

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
