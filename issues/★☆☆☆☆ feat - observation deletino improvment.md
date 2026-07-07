# feat - observation deletion improvment
Currently when a client is removed from var observations, all the observations after the removed one are shifted to the previous position. This may cause performance issues when there are many observations.

[controller_clienthelper.go](../sources/controller/controller_clienthelper.go#L195)

it can see in the file `sources/controller/controller_clienthelper.go`, inside the function `UnregisterObservation(varName string)`:

``` go
func (h *ControllerClientHelper) UnregisterObservation(varName string) {
	varName = "vars." + varName
	_ = misc.NamedLockRun("db.intenal.clients", 5*time.Second, func() {
		h.UpdateLiveTime()
		tcurr := h.db.Get("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(0))
		curr := int(tcurr.GetInt64())
		idx := h.findVarIndexOnObservingVars(varName)
		if idx > -1 {
			for c := idx; c < curr-1; c++ {
				next := h.db.Get("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(c+1), misc.NewDynamicVar(""))
				h.db.Set("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(c), next)
			}
			h.db.DeleteValue("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(curr-1), false)
			h.db.Set("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(curr-1))
		}
	})
}
```

## proposal
Instead shift all next observations to the previous position, just move the last observation to the removed one position and decrease the count of observations.