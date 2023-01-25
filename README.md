# grmgr
GoRoutine ManaGeR, grmgr, enables you to place a ceiling, using a `limiter` object, on the number of concurrent instances of a goroutine. Any number of limiters can be created.

grmgr comes in two editions, one which captures runtime metadata to a database in near realtime (build tag "withstats") and one without metadata reporting (no tag).
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
