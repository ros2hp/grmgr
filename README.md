# grmgr
GoRoutine ManaGeR, grmgr, enables you to place a ceiling (called a `limiter`) on the number of concurrent runtime instances of a goroutines. Any number of limiters can be created.

grmgr comes in two editions, one which captures runtime metadata to a database in near realtime (build tag "withstats") and one without metadata reporting (no tag).

For example to limit the number of concurrent `processDP` goroutines to no more than 10.

```
		// start grmgr service
		go grmgr.PowerOn(ctx, wpStart, wpEnd) 
		
		// create limiter for DP process setting ceiling to 10.
		limiterDP := grmgr.New("dp", 10)
		
		for node := range ch {
			
			// may block and wait for number of concurrent instances of goroutine 'processDP' to drop below ceiling.
			limiterDP.Control()
			
			go processDP(limiterDP, node)
		}
		
		. . .
		
		func processDP(l *grmgr.Limiter, node Node) {
			
			// signal to grmgr service that a goroutine has finished
			defer l.EndR()
			. . .
		}
```


One which reports in near realtime on the status of each limiter and one that does not.

1. Start a grmgr service users various channels to serialises access to shared data
```
    var wpEnd, wpstart sync.WaitGroup
  
	grCfg := grmgr.Config{"runid": runid}
	
	go grmgr.PowerOn(ctx, &wpStart, &wpEnd, grCfg) 
```
2. Create a grmgr limiter, assigning it a label and a ceiling representing the upper limit of concurrent goroutines.
```
	limiterDP := grmgr.New("dp", *parallel)
```
3. User the following code to block until grmgr notifies you that you can now instantiate a goroutine

```	
	limiterDP.Control()
```
4  Instantiate a goroutine passing the limiter as an argument

	go Propagate(ctx, limiterDP, &dpWg, u.PKey, ty, has11)

5. In the groutine issue the following code to notify grmgr service that the goroutine has termintaed
```
	defer limiterDP.EndR()
```
6. When nolonger required unregister the limiter using:
```
	limiterDP.Unregister()
```

Example code illustrating all four steps:

```
	//
	// start services
	//
	wpEnd.Add(3)
	wpStart.Add(3)
	//grCfg := grmgr.Config{"dbname": "default", "table": "runstats", "runid": runid}
	grCfg := grmgr.Config{"runid": runid}
	go grmgr.PowerOn(ctx, &wpStart, &wpEnd, grCfg) // concurrent goroutine manager service
	go errlog.PowerOn(ctx, &wpStart, &wpEnd)       // error logging service
	//go anmgr.PowerOn(ctx, &wpStart, &wpEnd)        // attach node service
	go monitor.PowerOn(ctx, &wpStart, &wpEnd) // repository of system statistics service
	wpStart.Wait()
	// blocking call..
	
	...
		limiterDP := grmgr.New("dp", *parallel)
		
		err := ptx.ExecuteByFunc(func(ch_ interface{}) error {

		ch := ch_.(chan []UnprocRec)
		var dpWg sync.WaitGroup

		for qs := range ch {
			// page (aka buffer) of UnprocRec{}

			for _, u := range qs {

				ty := u.Ty[strings.Index(u.Ty, "|")+1:]
				dpWg.Add(1)

				limiterDP.Ask()
				<-limiterDP.RespCh()

				go Propagate(ctx, limiterDP, &dpWg, u.PKey, ty, has11)

				if elog.Errors() {
					panic(fmt.Errorf("Error in an asynchronous routine - see system log"))
				}
			}
			if elog.Errors() {
					panic(fmt.Errorf("Error in an asynchronous routine - see system log"))
				}
			}
			// wait for dp ascyn routines to finish
			// alternate solution, extend to three bind variables. Requires MethodDB change.
			dpWg.Wait()
		}
		return nil
	})

	limiterDP.Unregister()
```
```
  
  
