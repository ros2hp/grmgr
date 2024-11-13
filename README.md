# grmgr - GoRoutine ManaGeR

## Concurrent Programming and grmgr?
 
Go's CSP (Communicating Sequential Processes) features, **_goroutines_** and **_channels_**, can effortlessly implement all the concurrent design patterns available. **_grmgr_** concerns its with the parallel design pattern which splits the data across a pool of concurrent goroutines each performing the identical processing logic. The number of **_goroutines_** that are concurrently running is a measure of the components parallelism, termed the **_degree of parallelism_** (dop) of the component. This is usually constrained to not exceed some maximum value to safeguard the system resources. An application may consist of many components employing both forms of concurrent patterns as well as hybrids of each e.g. pipeline consisting of steps that use the parallel pattern to increase throughput. Both patterns enable a Go application to scale across multiple cores/cpus.  

**_grmgr_** provides the parallel pattern with a throttle that can dynamically change, in realtime, the degree of parallelism (dop) of each parallel component from zero to a configured maximum. For example, **_grmgr_** can enable a paralle component to maintain a maximum **_dop_** of 20 and  respond to some external event, such as a CPU overload event, by reducing the **_dop_** by some margin, until the CPU overload event has passed and the **_dop_** can then return back to its original value using some predefined profile of increments. 

**_grmgr_** therefore enables your parallel components to intelligently scale up or down.

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
 	// throttle parallelTask to a maximum ot 10 concurrent instances
	const MAX_CONCURRENT = 10
       
	var throttle = make(chan,struct{},MAX_CONCURRENT)                 

	for node := range channel {

		throttle <- struct{}{}

		go { parallelTask()

		   <-throttle
                   }
	}
	 . . .

```

Using nothing more than a buffered channel, the above code has constrained the program's **dop** to not exceed 10. 

**grmgr** takes **dop** management to the next level however, enabling the  **dop**  to be dynamically increased or decreased.

## Auto-Scaling an Application using grmgr

**_grmgr_** has the ability to **_dynamically throttle_** each parallel component in your program in real-time. It does this by changing the **_dop_** of each parallel component, up or down, usually in response to some application or system resource monitoring event. The ability to scale an application dynamically like this represents a powerful application management capability permitting a programs resource consumption to be aligned with other applications running on the server or servers. 

Currently **_grmgr_** has no hooks into system monitors that might be used to trigger scale up or down events, however for applications running in the cloud all that is required is to identify the relevant cloud service and engage with its API. In the case of AWS for example, a single API setup in the SNS service is all that is required to get grmgr to respond to CloudWatch scaling alerts. 

**_grmgr_** can also send regular **_dop_** status reports to an "application dashboard". In fact **_grmgr_** has its own internal **_dop_** monitor for each parallel component that is persisted to a table in **_Dynamodb_** every five seconds.

Scaling events are sent to **_grmgr_** on dedicated channels. The contents of the message includes the name of the parallel component and the type of scaling event, either a specific  **_dop_* value or the name of a predefined scaling up or down profile.  If the message only contains the later, then all parallel components managed by  **_grmgr_** will be affected.

## **_grmgr_** Startup and Shutdown

**_grmgr_** runs as an asychronous background service to the application, meaning it runs as a goroutine communicating via dedicated channels. It would typically be started as part of the application initialisation and shutdown just before the application exits.  Just like a http server, **_grmgr_** listens for requests received on multiple channels and responds to them serially.  This maintains the intergrity of **_grmgr_** state data.

A **_grmgr_** service is started using the **_PowerOn()_** method, which accepts a Context and two WaitGroup instances.


```
	// WaitGroups to synchronise main with each service.
	var wpStart, wpEnd sync.WaitGroup 
	wpStart.Add(1)                         	
	wpEnd.Add(1)				
        . . .
	ctx, cancel := context.WithCancel(context.Background())   

	// start grmgr
 	go grmgr.PowerOn(ctx, &wpStart, &wpEnd) 
        . . .
	// wait for all servies to start
	wpStart.Wait()                          .
        . . .
	// shutdown grmgr using cancel() from context 
	cancel()                                 

```

All communication with the **_grmgr_** service is via channels which have been encapsulated in all **_grmgr_** method calls. This means the developer never needs to explicitly communicate with any channel associated with grmgr. For example, the **_Control()_** method implements the throttle feature (see next section) and it encapsulates all the necessary channel communications. 

```
	func (l Limiter) Control() {
		rAskCh <- l.r
		<-l.ch
	}
```


## Using grmgr to Auto-scale a Parallel Component

The code below has refactored the example from Section 1 with **_grmgr_**. Note the counter variable and channel have been removed as these features are encapsulated in **_grmgr_**.


```
	. . .
	// create a throttle with a DOP value of 10
	throttleDP := grmgr.New("data-propagation", 10)  
	
	for node := range ch {                          

		// blocking call to grmgr
		throttleDP.Control()               	
				
		go { processDP( node)
		     throttleDP.Done()
                   }    

	}
	// wait for all parallelDP's to finish
	throttleDP.Wait()                          	

	. . .
	// delete the throttle
	throttleDP.Delete()                          	

```

The **_New()_** function will create a grmgr throttle, which accepts both a name, which must be unique across all parallel components used in the application, and a maximum **_dop_** value. The throttling capability is handled by the **_Control()_** method. It is a blocking call and will wait for a response from the **_grmgr_** service before continuing.  The throttle's Done() method notifies **_grmgr_** that a gortouines has finished. In this way **_grmgr_** constraints the number of concurrent **_goroutines_** from exceeding the **_dop_**. 

A Throttle also comes equipped with a Wait() method, which emulates Go's Standard Library, sync.Wait(). In this case it will block and wait for all **_processDP_** goroutines that are still running to finish.

```
	throttleDP.Wait() 
```     

 
When a Throttle is no longer needed it should be deleted using:

```
	throttleDP.Delete()
```


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

