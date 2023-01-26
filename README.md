# grmgr
GoRoutine ManaGeR, **_grmgr_**, will maintain a fixed number of concurrent goroutine when it is instantiated in a for-loop.

In the example below a stream of potentially thousands of nodes is read from a channel. Each node is passed into processDP which is executed as a goroutine.

A grmgr limiter (limiterDP) is created with a value of 10, which sets the ceiling for the number of concurrent goroutines.

The Control() method will block when the number of concurrent groutines reaching the ceiling value. It will be unblocked when one of the groutines finishes. 
In this way **_grmgr_** will maintain the number of concurrent groutines at the ceiling value.

```

		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
	
			limiterDP.Control()  // blocking call
			
			go processDP(limiterDP, node)
		}
		limiterDP.Wait()
```
 **_grmgr_** runs as a service, which is a groutine that runs for the duration of the program. The service serialises access to the shared data of each of the active Limiters.
 
 **_grmgr_** comes in two editions, one which captures runtime metadata to a database in near realtime (build tag "withstats") and one without metadata reporting (no tag).
grmgr without reporting of metadata is sufficient for all cases outside of grmgr development.

For example to limit the number of concurrent `processDP` goroutines to no more than 10.

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
		// wait for DP goroutines to finish
		limiterDP.Wait()
		
		// free up limiter
		limiterDP.Delete()
		
		. . .
		
		func processDP(l *grmgr.Limiter, node Node) {
			
			// signal to grmgr service that a goroutine has finished
			defer l.EndR()
			. . .
		}
		
		// shudown grmgr service
		cancel()
		
		
```

## In Development

A throttler for the limiter. Set a range of values within which the limiter ceiling can operate. 

Provide a limiter.Up() and limiter.Down() to change the limiter's current ceiling value. 

