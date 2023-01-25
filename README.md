# grmgr
GoRoutine ManaGeR controls the number of concurrent instances of a goroutine. 

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
3. Block until grmgr notifies you that you can now instantiate a goroutine

```	
	limiterDP.Ask()
	<-limiterDP.RespCh()
```
4. When nolonger required unregister the limiter using:
```
	limiterDP.Unregister()
```

Example code illustrating all four steps:

```
	// blocking call..
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
  
  
