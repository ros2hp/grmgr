# grmgr - goroutine manager

## Concurrent Programming and grmgr?
 
There are two distinct patterns in concurrent programming, both of which are readily implemented using Go's CSP (Communicating Sequential Processes) features, **_goroutines_** and **_channels_**. The first pattern is the **_process pipeline_**, which splits some processing logic into smaller units and deploys each unit as a goroutine linked together by channels to form a processing pipeline. Data is passed from one goroutine to the next goroutine via their associated channel until the data exits the pipeline in its fully processed state. The other pattern is **_parallel_concurrency_** which splits the data, rather than the processing logic, across a pool of concurrent goroutines each performing the identical processing logic. The number of **_goroutines_** that are concurrently running is a measure of the components parallelism, termed the **_degree of parallelism_** (dop) of the component. This is usually constrained to not exceed some maximum value to safeguard the system resources. An application may consist of many components employing both forms of concurrent patterns as well as hybrids of each e.g. pipeline consisting of steps that use the parallel pattern to increase throughput. Both patterns enable a Go application to scale across multiple cores/cpus.  

**_grmgr_** is not concerned with pipelines, however it provides the parallel processing pattern with a throttle that can dynamically adjust, in realtime, the degree of parallelism of each parallel component from zero to a configured maximum value. For example, **_grmgr_** can enable a paralle component to maintain a maximum **_dop_** of 20 and then respond to some external event, such as a CPU overload event, by reducing the **_dop_** by some margin, until the CPU overload event has passed and the **_dop_** can then return back to its original value using some predefined profile of increments. 

**_grmgr_** therefore enables your parallel components to intelligently scale by responding to external events, such as an alert from a system monitor or an internal applicaiton metric.

## Parallel Processing Examples in Go.

The code fragment below presents a naive implementation of the parallel processing pattern. You will note there is no attempt to limit the number of concurrent **_parallelTask_** that are instantiated. The **_dop_** at any moment in time is dependent on two factors, one, the number of entries (nodes) queued in the channel buffer and secondly, how long the parallelTask operations takes to run. Consequently the program may generate a widely unpredicatable load on the server which is not good from the perspective of other running programs.

```	. . .
	var channel = make(chan,node)
	.. . .
	for node := range channel {

		go parallelTask(node)  // non-blocking. parallelTask starts executing immediately as a goroutine.

	}
	. . .
```

A relatively easy fix to the above is to constrain the number of **_parallelTask_** that can run concurrently. This is achieved by adding a "counter", to count the number of instantiated goroutines, and a **_channel_**, to synchronise the completion of a **_parallelTask_** with the main program permiting another task to run after some threshold of concurrent tasks has been satisfied. 

```
	. . .
	const MAX_CONCURRENT = 10              // throttle parallelTask to a maximum ot 100 concurrent instances
	var channel = make(chan,message,MAX_CONCURRENT)   // channel to send back finish message
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

Using no more than a counter and a channel, the above code has stabilised the program by constraining the number of concurrent **_parallelTasks_** to never exceed 10. 

**grmgr** takes **dop** management to the next level however, enabling the  **dop**  to dynamically increase or decrease under the influence of some external event and enable your application to dynamically scale to suite the prevailing system resources.

## Auto-Scaling an Application using grmgr Dynamic Throttling

As mentioned, **_grmgr_** has the ability to **_dynamic throttle_** each parallel component in real-time while the application is running. It can do this by changing the **_dop_** of each parallel component, up or down, usually in response to some application or system scaling event. The ability to scale an application dynamically in real-time represents a powerful application management capability permitting an application's resource consumption to be aligned  with other applications running on the server. 

**_grmgr_** has no hooks into system monitors that might be used to trigger scale up or down events, however for applications running in the cloud all that is required is to identify the relevant cloud service and engage with its API. In the case of AWS for example, *_grmgr_**  would only need to implement a single API from the SNS service which would give it the ability to respond to CloudWatch scaling alerts. Very easy. 

**_grmgr_** can also send regular **_dop_** status reports to an "application dashboard". In fact **_grmgr_** has its own internal **_dop_** monitor for each parallel component which is persisted to a table in **_Dynamodb_** every few seconds.

Scaling events are sent to **_grmgr_** on a dedicated channel. The contents of the message includes the name of the parallel component and the type of scaling event, either a scalar  **_dop_* value to attain as quickly as possible or the name of a predefined scaling up or down profile.  If the message only contains a scaling event then all parallel components managed by  **_grmgr_** will be affected.

## **_grmgr_** Startup and Shutdown

**_grmgr_** runs as a "asychronous background service" to the application, meaning it runs as a goroutine communicating with the outside world via a number of channels. It would typically be started as part of the application initialisation and shutdown just before the application exits.  Just like a http server, **_grmgr_** listens for requests received on multiple channels and responds to them serially.  This maintains the intergrity of **_grmgr_** state data as it handles requests from multiple parallel components and external systems concurrently.

A **_grmgr_** service is started using the **_PowerOn()_** method, which accepts a Context and two WaitGroup instances.


```
	var wpStart, wpEnd sync.WaitGroup       // create a pair of WaitGroups instances, that are used
						// to synchronise the main program with the startup of each service. 

	wpStart.Add(?)                         	// ? represents the number of services being started
	wpEnd.Add(?)				// one of which will be grmgr.
        . . .
	ctx, cancel := context.WithCancel(context.Background())    // create a context. Used to terminate the grmgr service

 	go grmgr.PowerOn(ctx, &wpStart, &wpEnd)  		   // start grmgr
        . . .
	wpStart.Wait()                          // wait for all servies to start.

```


To shutdown the service execute the cancel function generated using the **_WithCancel_** method used to create the context that was passed into the **_PowerOn()_**.


```
	cancel() 
```

All communication with the **_grmgr_** service is via channels which have been encapsulated into all **_grmgr_** method calls. This means the developer never explicitly communicates with a channel.   


## Using grmgr to Manage a Parallel Component

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

