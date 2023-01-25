# grmgr
GoRoutine ManaGeR controls the number of concurrent instances of a goroutine. 

First start a grmgr service which serialises access to shared data
```
  var wpEnd, wpstart sync.WaitGroup
  
	grCfg := grmgr.Config{"runid": runid}
	go grmgr.PowerOn(ctx, &wpStart, &wpEnd, grCfg) 
```
Control the number of concurrent instances of a goroutine using the following:

```
limiterDP := grmgr.New("dp", *parallel)

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
			// wait for dp ascyn routines to finish
```
  
  
