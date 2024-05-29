# grmgr - goroutine manager

## Concurrent Programming and grmgr?
 
 
There are two distinct patterns in concurrent programming, both of which are readily implemented using Go's CSP features, **_goroutines_** and **_channels_**. The first pattern is the **_process pipeline_**, which splits some processing logic into smaller units, where each unit is executed in its own goroutine which is then linked via channels to the next goroutine forming a pipeline of goroutines that in aggregate perform the complete processing logic on the data. Data is then passed from one goroutine to the next goroutine via a channel until it exists the pipeline. The other pattern is **_parallel_concurrency_** which splits the data, rather than the processing logic, across a pool of concurrent goroutines each performing the identical processing logic. The number of **_goroutines_** that are concurrently running is a measure of the components parallelism. This is usually constrained to not exceed some maximum value which is then termed the **_degree of parallelism_**  (dop) of the component. An application may consist of many components employing the parallel patter, each configured with its own **_dop_**.  

Both concurrent programming patterns enable an application to scale across multiple CPUs/cores. However, some scaling can also be achieved on single cpu/core systems when there are blocking operations, like file io or database requests, involved.  

**_grmgr_** is not concerned with pipelines. grmgr is used in parallel processing patterns to enforce the **_degree of parallelism_** of the component. For example, it is possible to configure grmgr to maintain a **_dop_** of 20 for a component and it will ensure the application cannot allocate more than 20 concurrent instances of the function.  

Let's look at a coding example using the parallel processing pattern when not using grmgr. The code fragment below presents a naive example of parallel processing as it will instantiate an unknown number of concurrent **_parallelTask_** functions. The **_dop_** of the operation will depend on how many nodes are queued in the channel at any point in time and how long the paralleTask operations takes to run. For this reason the **_dop_** may vary from 0 to a large unknown number throughout the life of the application. For this reason the code is considered naive as there is no checks or control over how many **_parallelTask_** are running before the next loop instantiates another. 

```	. . .
	var channel = make(chan,node)
	.. . .
	for node := range channel {

		go parallelTask(node)  // non-blocking. parallelTask starts executing immediately as a goroutine.

	}
	. . .
```

The repercussions of a potentially unlimited **_dop_** should be pretty obvious. It has the potential to place a strain on the server resources like CPU or memory when the **_dop_** is very high. At any point it may also  exceed the number of database connections if a database request is performed in the operation. The variable and unpredictable and potentially hostile consumption of system and database resources therefore makes for an anti-social application that cannot safely coexist with other applications.

To introduce some control over the **_dop_** (aka throttling) of the **_parallelTask_** is fortunately quite easy. Simply add a "counter" and create a  **_channel_** so **_parallelTask_** can pass back a "finished" message.

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

Using no more than a combination of counter and channel, the above code has stabilised the consumption of resources by constraining the number of concurrent parallelTasks to not exceed 100. However what if you want to be able to vary the **_dop_** from 100 to 20, or 100 to 150, while the application is running? How might the developer introduce some level of **_dynamic throttling_** to the application? 

## Why the need for grmgr?

While **_grmgr_** may be used to throttle all parallel goroutines within an application it also provides the facility to dynamically adjust the parallel limits of each goroutine, up or down, and in realtime while the application is running. grmgr can then implement those changes via the usual communication with the application, so as the applicaton continues to run the number of concurrent instances of each goroutines will be brough into line to match the new modified limits. 

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
