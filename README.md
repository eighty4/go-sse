## SSE Upgrade

This library exports a func that upgrades http requests to an event streaming connection by sending a `Content-Type: text/event-stream` header to the client.
```go
http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
    var connection *sse.Connection
    connection, _ = sse.Upgrade(w, r)
    connection.SendString("thanks for connecting. here's some data.")
})
```

Connection's `SendBytes([]byte)`, `SendString(string)` and `SendJson(interface{})` funcs will format and send data to the client.

Use `Close()` when you're done streaming event data.

A connection's BuildMessage() func can be used to send a payload with the id and event attributes.
```go
connection.BuildMessage().WithId("id").WithEvent("event").SendString("data")
```
This message will be sent to the client:
```text
id: id
event: event
data: data

```

### Example
```go
package main

import (
	"github.com/eighty4/sse"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	connection, err := sse.Upgrade(w, r)
	if err != nil {
		w.WriteHeader(500)
		log.Println("sse upgrade error", err.Error())
		return
	}
	connection.BuildMessage().WithEvent("auth").SendString("token")
	connection.Close()
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

#### Original Repository
I found originating source for sse.go on GitHub a couple of years ago, but I couldn't find the repository to reference when publishing updates.
