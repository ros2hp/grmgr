# grmgr
Whever there is an opportunity to instantiate an unknown number of goroutines, typically in a for-loop, **_grmgr_** (GoRoutine ManaGeR) can be used to constrain the number of concurrent goroutines to a fixed and constant number until it finishes. 

In the example below we have a typical scenario where a stream of potentially thousands of nodes is read from a channel. Each node is passed to a goroutine, processDP().

Rather than instantiate thousands of concurrent goroutintes the developer can create a  **_grmgr_** Limiter, specifying a ceiling value, and use the Control() method in the for-loop to constrain the number of concurrent goroutines.

```

		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
	
			limiterDP.Control()  // blocking call
			
			go processDP(limiterDP, node)
		}
		limiterDP.Wait()
```
The Control() method will block when the number of concurrent groutines, __processsDP__ in this case, equals the ceiling value of the Limiter. When a goroutine finishes Control() will be unblocked.
In this way **_grmgr_** can maintain a fixed and constant number of concurrent goroutines, equal to the ceiling value. The Limiter also comes with a Wait(), which emulates sync.Wait(), and in this case will wait until the last of the goroutines finish.

To safeguards the state of each Limiter, **_grmgr_** runs as a service, meaning  **_grmgr_** runs as a goroutine and communicates only via channels (encapsulated in the methos) so access to shared data is serialised and therefore safe.
So before using  **_grmgr_**  you must start the service using:

```
 		go grmgr.PowerOn(ctx, wpStart, wpEnd) 
```
Where wpStart and wpEnd are instances of sync.WaitGroup used to synchronise when the service has been started and shutdown via the context ctx. The  **_grmgr_** handles an unlimited number of Limters.
 
When the Limiter is nolonger needed it should be deleted:

```
	limiterDP.Delete()
```

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

