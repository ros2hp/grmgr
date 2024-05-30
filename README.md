# grmgr - goroutine manager

## Concurrent Programming and grmgr?
 
There are two distinct patterns in concurrent programming, both of which are readily implemented using Go's CSP features, **_goroutines_** and **_channels_**. The first pattern is the **_process pipeline_**, which splits some processing logic into smaller units and deploys each unit as a goroutine linked together using channels to form a pipeline. Data is passed from one goroutine to the next goroutine via their associated channel until the data exits the pipeline in its fully processed state. The other pattern is **_parallel_concurrency_** which splits the data, rather than the processing logic, across a pool of concurrent goroutines each performing the identical processing logic. The number of **_goroutines_** that are concurrently running is a measure of the components parallelism. This is usually constrained to not exceed some maximum value which is then termed the **_degree of parallelism_**  (dop) of the component. An application may consist of many components employing the parallel pattern, each configured with its own **_dop_**. 

Both concurrent patterns enable an application to scale across multiple CPUs/cores. However, some scaling may also be possible on single cpu/core systems if there are blocking operations, like file io or database requests, involved.  

**_grmgr_** is not concerned with pipelines. **_grrmgr_** is used in parallel processing patterns to enforce the **_degree of parallelism_** of the component. For example, it is possible to configure **_grrmgr_** to maintain a **_dop_** of 20 for a component and it will ensure the application cannot allocate more than 20 concurrent instances of the function.  

The code fragment below presents a naive implementation of the parallel processing pattern when not using **_grrmgr_**. You will note in the for-loop it does not implement any checks over the number of concurrent **_parallelTask_** that are running before the next execution of the loop instantiates another one. The **_dop_** of this operation will depend on the number of nodes queued in the channel buffer, at any point in time, and how long the parallelTask operations take to run. For this reason the **_dop_** may vary from 0 to an unknown and potentially very large number. 

```	. . .
	var channel = make(chan,node)
	.. . .
	for node := range channel {

		go parallelTask(node)  // non-blocking. parallelTask starts executing immediately as a goroutine.

	}
	. . .
```

The repercussions of a potentially unlimited **_dop_** should be obvious. It has the potential to place a strain on the server resources like CPU or memory when the **_dop_** is very high. If the task performs a database request it may also exceed the number of database connections at the database or in the application's pool of connections. The variable and unpredictable and potentially hostile consumption of system and database resources therefore makes it an anti-social application that cannot safely coexist with other applications.

To introduce some control over the **_dop_** of the **_parallelTask_** is fortunately quite easy. A common expression for this capability is **_throttling_**. To throttle this component is a matter of adding a "counter" and a  **_channel_**. The channel is used to send a "finished" message from **_parallelTask_**.

```
	. . .
	const MAX_CONCURRENT = 100              // throttle parallelTask to a maximum ot 100 concurrent instances
	var channel = make(chan,message)        // create a Go channel to send back finish message from parallelTask
	task_counter := 0                 

	for node := range channel {

		go parallelTask(node, channel)  // instantiate parallelTask as a goroutine. Pass in the channel.
		task_counter++

		if task_counter == MAX_CONCURRENT {

			<-channel               // block and wait for a finish message from any one of the running parallelTask's
			task_counter--
		}
	}
	 . . .

```

Using no more than a counter and a channel, the above code has stabilised the consumption of resources by constraining the number of concurrent **_parallelTasks_** to never exceed 100. However what if we want to vary the **_dop_** from 100 to 20, or 100 to 150, while the application is running? How might the developer introduce some level of **_dynamic throttling_** to the application? Hint, it's not trivial. 

## Auto-Scaling an Application using Dynamic Throttling

**_grmgr_** has the ability to **_dynamic throttling_** each parallel component in real-time while the application is running. It can do this by changing the **_dop_** of each parallel component, up or down, usually in response to some application scaling event. The event might be sourced from a server monitor that has been triggered by a CPU or memory alarm, or an internal application monitor responding to a queue size alarm. The ability to scale an application dynamically in real-time represents a powerful system's management capability and means the application's resource consumption can be varied to better align it with other applications running on the server, making it a good neighbour program. 

The current version of **_grmgr_** has no hooks into system monitors. However for applications running in the cloud this would be very simple matter of identifying the relevant cloud service and engaging with the API. In the case of AWS, *_grmgr_**  would only need to implement a single API from the SNS service which would give it the ability to respond to CloudWatch scaling alerts. Too easy. 

**_grmgr_** could also send regular **_dop_** status reports to an "application dashboard" for display purposes or recording in a database for later analysis.

## **_grmgr_** Startup and Shutdown

**_grmgr_** runs as a "background service" to the application, which simply means it runs as a goroutine. It would typically be started as part of the application initialisation and shutdown just before the application exits. 

A **_grmgr_** service is started using the **_PowerOn()_** method, which accepts a Context and two WaitGroup instances.


```
	var wpStart, wpEnd sync.WaitGroup                          // create a pair of WaitGroups instances

	wpStart.Add(?)                                             // define a value for each WaitGroup
	wpEnd.Add(?)

	ctx, cancel := context.WithCancel(context.Background())    // create a context.


 	go grmgr.PowerOn(ctx, &wpStart, &wpEnd)                    // start grmgr as a goroutine


```


To shutdown the service execute the cancel function generated using the **_WithCancel_** method used to create the context that was passed into the **_PowerOn()_**.


```
	cancel() 
```

All communication with the **_grmgr_** service uses channels which have been encapsulated in the **_grmgr_** method calls, so the developer does not use any channels explicitly.  


## Using grmgr to manage a parallel task

The code example below has introduced **_grmgr_** to the example from Section 1. Note the lack of counter and channel as both have been encapsulated into **_grmgr_**.


```
	. . .
	throttleDP := grmgr.New("data-propagation", 10)  // create a throttle with dop 10
	
	for node := range ch {                     
	
		throttleDP.Control()               	// wait for a response from grmgr
				
		go processDP(throttleDP, node)      

	}

	throttleDP.Wait()                          	// wait for all parallelDP's to finish

	. . .
	
	throttle.Delete()                          	// delete the throttle

```

The **_New()_** function will create a throttle, which accepts both a name, which must be unique across all throttles used in the application, and a **_dop_** value. The throttling capability is handled by the **_Control()_** method. It is a blocking call which will wait for a response from the **_grmgr_** service before continuing.  It will immediately unblock when the number of running **_processDP_** is less than or equal to the **_dop_** defined in New(). If the number is greater than the **_dop_** value it will wait until one of the **_processDP_** goroutines has finished. In this way **_grmgr_** constraints the number of concurrent goroutines to be no more than the **_dop_** value. 

The code behind the Control() method illustrates the encapsulated channel communicate with the **_grmgr_** service. 

```
	func (l Limiter) Control() {
		rAskCh <- l.r
		<-l.ch
	}
```

 A Throttle also comes equipped with a Wait() method, which emulates Go's Standard Library, sync.Wait(). In this case it will block and wait for all **_processDP_** goroutines that are still running to finish.

```
	throttleDP.Wait() 
```     

 
When a Throttle is no longer needed it should be deleted using:

```
	throttleDP.Delete()
```

## Modifying your parallel function to work with grmgr.

The function must accept a grmgr throttle instance and include the following line of code, usually placed at or near the top of the function.


```
	defer limiterDP.Done()
```

The Done() method will notify grmgr that the goroutine has finished.



To configure a logger for __**_grrmgr_**__ use the following:
```
	**_grrmgr_**.SetLogger(<logger>, <log level>) 
```
Available log levels are:
```
	const (
		Alert LogLvl = iota
		Debug
		NoLog
	)
```

## Compiler Options

 **_grrmgr_** comes in two editions, one which captures runtime metadata to a database in near realtime (build tag "withstats") and one without metadata reporting (no tag).


## Configuring the Throttle

To configure a non-default throttle use NewConfig().


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
	


## Modify the **_dop_** 

Use the following methods on a throttle to vary the throttle's **_dop_** value based on the throttle configuration.


```
	Up()

	Down()
```

