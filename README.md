# grmgr
GoRoutine ManaGeR, **_grmgr_**, will maintain a fixed number of concurrent instances of a particular goroutine when it is instantiating in a for-loop.

In the example below a stream of potentially thousands of nodes is read from a channel. Each node is passed into processDP which is executed as a goroutine.

A limiterDP is created with a ceiling value fo 10 which will maintain the number of concurrent goroutines at between 9 and 10 at any moment in time, until the last 10 nodes is reached.

The Control() method will block when the number of concurrent groutines exceeeds 10. It will be unblocked when one of the groutines finishes. 
In this way it can maintain the number of concurrent groutines at the ceiling value.

```

		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
	
			limiterDP.Control()
			
			go processDP(limiterDP, node)
		}
		limiterDP.Wait()
```

**_grmgr_** will maintain the number of concurrent goroutines to between the ceiling and ceiling-1 at all times until they wind down.



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

