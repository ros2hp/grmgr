# grmgr
Whever there is an opportunity to instantiate an unknown number of goroutines, typically in a for-loop, **_grmgr_** (GoRoutine ManaGeR) can be used to constrain the number of concurrent goroutines to a fixed and constant number until it finishes. 

In the example below we have such a typical scenario where a stream of potentially thousands of nodes is read from a channel. Each node is passed to a goroutine, processDP().

Rather than instantiate thousands of concurrent goroutintes the developer can create a  **_grmgr_** Limiter, specifying a ceiling value, and use the Control() method in the for-loop to constrain the number of concurrent goroutines.

```

		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
	
			limiterDP.Control()  // blocking call
			
			go processDP(limiterDP, node)
		}
		limiterDP.Wait()
```
The Control() method will block when the number of concurrent groutines, processsDP in this case, equals the ceiling value of the Limiter. When a goroutine finishes Control() will be unblocked.
In this way **_grmgr_** can maintain a fixed and constant number of concurrent goroutines. The Limiter also comes with a Wait(), which emulates sync.Wait(), and will wait until the remaining goroutines finish.

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

