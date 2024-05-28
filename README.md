# grmgr - goroutine manager

## A quick 101 on CSP?

Built into Go are the components that enable a style of programming called Communicating Sequential Processes (CSP), which in the humble opinion of the author is the language's most identifying and powerful feature. The first components is a **_goroutine_**, which is a function that runs asynchronously via Go's runtime scheduler. The second fundamental component is a **_channel_**, which provides the infrastructure to enable communication between **_goroutines_**, i.e. enabling the C in CSP. CSP enables a pipeline of goroutines to be created with channels between each routine in the pipeline exchanging data. This is an example of concurrently running goroutines, they perform different tasks but are running concurrently.
A different example of concurrency is where multiple instances of the same gofunction are running concurrently. This is known as parallel processing. This actual number of instances running in parallel is usually fixed and is known as the degree-of-parallelism of the software component that is running the parallel goroutines. grmgr is only concerned with parallel concurrency. It will ensure that only a fixed number of goroutines are running concurrently based on the degree of parallelism specified when the grmgr throttle was created. 

The code fragment below is an niave example of how to instantiate parallel processing of the **_parallelTask_** function.  In this example the for-loop reads node data from a channel populated by a goroutine that is not shown. The body of the for-loop instantiate  **_parallelTask_**  as a goroutine using the **_go_** keyword. 

```	. . .
	var channel = make(chan,node)
	.. . .
	for node := range channel {

		go parallelTask(node)  // non-blocking. parallelTask starts executing immediately

	}
	. . .
```

As the for-loop body has no constraints there as many **_parallelTask_** functions started as their are entries in the channel queue. If there is a lot this may impose a considerable load on the server, depending on how long the function takes to run, or it performs some database operations, it will eventually consume all the database connections available in the connection pool.  Consequently some control over the number of concurrent **_parallelTask_** needs to be introduced. We refer to this limit as the **_degree of parallelism_** of the goroutine.  The act of constraining the number of concurrent functions is referred to as **_throttling_**. 

To introduce some throttling on **_parallelTask_** is quite easy. Simply as adding a "counter" and create a  **_channel_** to pass back a "finished" message from each **_parallelTask_**.

```
	. . .
	const MAX_CONCURRENT = 100                  // throttle parallelTask to a maximum ot 100 concurrent instances
	var channel = make(chan,message)            // create a Go channel to send back finish message from parallelTask
	task_counter := 0                 

	for node := range channel {

		go parallelTask(node, channel)      // instantiate parallelTask as a goroutine. Pass in the channel.
		task_counter++

		if task_counter == MAX_CONCURRENT {

			<-channel                       // block and wait for a finish message from any one of the running parallelTask's
			task_counter--
		}
	}
	 . . .

```
So, if it is so easy to manage the number of concurrent **_goroutines_**, why the need for a package that claims to manager goroutines?

## The Benefits of grmgr?

While **_grmgr_** may be used to throttle many goroutines across an application it also provides the facility to dynamically adjust the throttle limits of each goroutine, up or down, in realtime while the application is running. As the applicaton continues to run the number of concurrent instances of each goroutines will be brought into line to match the new throttle limits. 

This capability could be used by a system monitor, for example, to send a scale-down message to  **_grmgr_** in response to a CPU overload alarm. Over time the application would adjust the number of concurrent goroutines in response to **_grmgr_** enforcing lower concurrency limits of each goroutine. 

Not only could **_grmgr_** respond to scaling events from external systems it could also feed data into an application dashboard displaying, for example, historic and current values of the  degree of parallelism of each **_goroutine_**. 

The benefits from **_grmgr_** therefore may be quite significant. However, **_grmgr_** does not currently support any monitoring systems or dashboard technolgies so its useful is somewhat limited, although it does provide its own performance data.

## Coding Examples using grmgr

In the code snippet below we have the same scenario from the previous example but this time written using **_grmgr_** to throttle the number of concurrent goroutines. Note how we don't have to create a channel or a counter. The throttle has subsumed this functionality internally.


```
	. . .
	throttleDP := grmgr.New("data-propagation", 10)  // create a grmgr throttle - set max concurrent to 10.
	
	for node := range ch {                     // wait for node data on channel.
	
		throttleDP.Control()               // wait for a response from grmgr to proceed.
			
		go processDP(throttleDP, node)     // non-blocking. Instantiate processDP as a goroutine. 

	}

	throttleDP.Wait()                          // wait for all processDP goroutines to finish.
	. . .
```

The Control() method will block and wit on a message from **_grmgr_** to proceed. **_grmgr_** will only send a message when the number of concurrent groutines is less than or equal to the max concurrent defined for the throttle. When any of the **_processDP_** goroutines finish Control() will be immedidately unblocked.
In this way **_grmgr_** can maintain a fixed number of concurrent goroutines. A Throttle also comes with a Wait() method, which emulates the sync.Wait() from the Standard Library. in this case it will wait for all the **_processDP_** goroutines to finish.

## Architecture and Setup

**_grmgr_** runs as a background "service" to the application. This simply means it runs as a goroutine which is typically started as part of the application initialisation and shutdown just before the application exits. 

Before using  **_grmgr_**  the service must be started:

```
	var wpStart, wpEnd sync.WaitGroup                              // create a pair of WaitGroups
	wpStart.Add(?)                                                 
	wpEnd.Add(?)

	ctx, cancel := context.WithCancel(context.Background())         // create a std package context with cancel function.

 	go grmgr.PowerOn(ctx, &wpStart, &wpEnd)                         // start grmgr as a goroutine
```

To shutdown the **_grmgr_** service simply issue the cancel function generated via the context package:

```
	cancel() 
```

All communication with the **_grmgr_** service is via channels that are encapsulated within various methods of a throttle instance, created using the grmgr.New() function. In this way access to the parallelism state of each goroutine is serialised and **_grmgr_** is therefore concurrency safe.

The code behind the Control() method  illustrates the use of channels to communicate with the **_grmgr_** service. 

```
	func (l Limiter) Control() {
		rAskCh <- l.r
		<-l.ch
	}
```
 
When a Throttle is nolonger needed it should be deleted:

```
	throttleDP.Delete()
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

To configure a non-default throttle use NewConfig()
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
