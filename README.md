# Kapi

RestAPI and RestAPI client simulator for Kabina in Go
## Running RestAPI client simulators

Edit *client/utils/utils.go* and adapt host address to the location of your RestAPI server, in the line that starts with *var host string = "* 

```
cd client
go build
```

It builds *kabina* executable, which can be run in two modes:
**./kabina cab** emulates cabs
**./kabina** emulates customers

Have a look at the start of *main.go* file, there are some parameters of simulation that can be adjusted. 
