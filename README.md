# grmgr - goroutine manager

## Concurrent Programming and grmgr?
 
There are two distinct patterns in concurrent programming, both of which are readily implemented using Go's CSP features, **_goroutines_** and **_channels_**. The first pattern is the **_process pipeline_**, which splits some processing logic into smaller units and deploys each unit as a goroutine linked together by channels to form a processing pipeline. Data is passed from one goroutine to the next goroutine via their associated channel until the data exits the pipeline in its fully processed state. The other pattern is **_parallel_concurrency_** which splits the data, rather than the processing logic, across a pool of concurrent goroutines each performing the identical processing logic. The number of **_goroutines_** that are concurrently running is a measure of the components parallelism, termed the **_degree of parallelism_** (dop) of the component. This is usually constrained to not exceed some maximum value to safeguard the system resources. An application may consist of many components employing both forms of concurrent patterns as well as hybrids of each e.g. pipeline consisting of steps that use the parallel pattern to increase throughput. Both patterns enable a Go application to scale across multiple cores/cpus.  

**_grmgr_** is not concerned with pipelines, however it provides the parallel processing pattern with a throttle that can dynamically adjust, in realtime, the degree of parallelism of each parallel component from zero to a configured maximum value. For example, **_grmgr_** can enable a paralle component to maintain a maximum **_dop_** of 20 and then respond to some external event, such as a CPU overload event, by reducing the **_dop_** by half, until the CPU overload event has passed and the **_dop_** can return back to its original value using some predefined profile. 

**_grmgr_** therefore enable your parallel components to intelligently scale by responding to external events such as system monitor alerts or a internal applicaiton alert, such, as increasing error rates in a subprocess that is related to throughput.

## Parallel Processing Examples in Go.

The code fragment below presents a naive implementation of the parallel processing pattern. You will note there is no attempt to limit the number of concurrent **_parallelTask_** that are instantiated. The **_dop_** at any moment in time is dependent on two factors. One, the number of entries (nodes) queued in the channel buffer and secondly, how long the parallelTask operations takes to run. Consequently the program may exhibit a widely unpredicatable load on the server which is not good from the perspective of other programs running on the server.

```	. . .
	var channel = make(chan,node)
	.. . .
	for node := range channel {

		go parallelTask(node)  // non-blocking. parallelTask starts executing immediately as a goroutine.

	}
	. . .
```

A relatively easy fix to the above is to constrain the number of **_parallelTask_** that can run concurrently to some maximum value. This is achieved by adding a "counter" and a  **_channel_**, to send a "finished" message back to the main program from each **_parallelTask_** goroutine.

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

Using no more than a counter and a channel, the above code has stabilised the program by constraining the number of concurrent **_parallelTasks_** to never exceed 100. 

** grmgr ** takes **dop** management to the next level, however, by enabling it to be dynamically increased or decreased under the influence of some external event and enable your application to dynamically scale to suite the prevailing system resources such as CPU or IO.

## Auto-Scaling an Application using grmgr Dynamic Throttling

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

The **_New()_** function will create a throttle, which accepts both a name, which must be unique across all throttles used in the application, and a **_dop_** value. The throttling capability is handled by the **_Control()_** method. It is a blocking call, meaning, it will wait for a response from the **_grmgr_** service before continuing.  **_grmgr_** will respond immediately when the number of running **_processDP_** is less than or equal to the **_dop_** defined in New(). If the number is greater than the **_dop_**  **_grmgr_** will not respond until one of the **_processDP_** has finished preventing another instantiate of the function. In this way **_grmgr_** constraints the number of concurrent **_goroutines_** from exceeding the **_dop_**. 

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

