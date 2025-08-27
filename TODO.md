# TODO

## Handle not GH dependencies

Some dependencies are not based on github, and return errors like this:
```
nhooyr.io/websocket - Invalid module path: invalid module path: nhooyr.io/websocket
google.golang.org/api - Invalid module path: invalid module path: google.golang.org/api
google.golang.org/grpc - Invalid module path: invalid module path: google.golang.org/grpc
```

Minimum change: these should be included in the output as 'unknown'

Better change: Try to resolve these and determine whether they are archived using other heuristics
