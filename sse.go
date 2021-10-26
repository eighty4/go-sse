// Package sse provides a utility for upgrading http requests to event streaming connections
// with server sent events (SSE).
package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

// Connection provides channels for sending event messages, closing the connection and
// receiving errors from writing to the http response
type Connection struct {
	errors   <-chan error
	messages chan<- Message
	shutdown chan<- bool
	isOpen   bool
}

// BuildMessage returns a MessageBuilder, a fluent-style builder api for sending events
func (connection *Connection) BuildMessage() *MessageBuilder {
	return &MessageBuilder{
		message:    &Message{},
		connection: connection,
	}
}

// SendBytes sends a series of bytes for an event's data without an id or event field
func (connection *Connection) SendBytes(data []byte) error {
	return connection.BuildMessage().SendBytes(data)
}

// SendString sends a string for an event's data without an id or event field
func (connection *Connection) SendString(data string) error {
	return connection.BuildMessage().SendString(data)
}

// SendJson marshals data into a json string for an event's data and sends it
// without an id or event field
func (connection *Connection) SendJson(data interface{}) error {
	return connection.BuildMessage().SendJson(data)
}

// IsOpen returns whether connection is still open for sending event data
func (connection *Connection) IsOpen() bool {
	return connection.isOpen
}

// Close sends a shutdown signal to close the connection for streaming data
func (connection *Connection) Close() {
	connection.shutdown <- true
}

func (connection *Connection) send(message *Message) error {
	if !connection.isOpen {
		return errors.New("connection is closed")
	}
	connection.messages <- *message
	return nil
}

// MessageBuilder is a fluent-style builder api for sending events
type MessageBuilder struct {
	message    *Message
	connection *Connection
}

// WithId adds an id attribute to event data
func (messageBuilder *MessageBuilder) WithId(id string) *MessageBuilder {
	messageBuilder.message.id = id
	return messageBuilder
}

// WithEvent adds an event attribute to event data
func (messageBuilder *MessageBuilder) WithEvent(event string) *MessageBuilder {
	messageBuilder.message.event = event
	return messageBuilder
}

// SendBytes sends a series of bytes with the specified id and event attributes
func (messageBuilder *MessageBuilder) SendBytes(data []byte) error {
	messageBuilder.message.data = data
	return messageBuilder.connection.send(messageBuilder.message)
}

// SendBytes sends a string with the specified id and event attributes
func (messageBuilder *MessageBuilder) SendString(data string) error {
	messageBuilder.message.data = []byte(data)
	return messageBuilder.connection.send(messageBuilder.message)
}

// SendJson marshals data into a json string and sends it without an id or event field
func (messageBuilder *MessageBuilder) SendJson(data interface{}) error {
	if data, err := json.Marshal(data); err != nil {
		return err
	} else {
		messageBuilder.message.data = data
		return messageBuilder.connection.send(messageBuilder.message)
	}
}

// Message contains id, event and data attributes of an event message
type Message struct {
	id    string
	event string
	data  []byte
}

// Upgrade sends headers to client to upgrade the request to an SSE connection and
// returns a Connection handle for sending messages.
func Upgrade(writer http.ResponseWriter, request *http.Request) (*Connection, error) {

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	errorChannel := make(chan error)
	messageChannel := make(chan Message)
	shutdownChannel := make(chan bool)
	sseConnection := &Connection{
		errors:   errorChannel,
		messages: messageChannel,
		shutdown: shutdownChannel,
		isOpen:   true,
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	handleError := func(err error) {
		if err != nil {
			select {
			case errorChannel <- err:
				break
			default:
				log.Println("sse write error: " + err.Error())
			}
		}
	}

	go func() {
		for {
			var err error
			select {
			case message := <-messageChannel:
				if len(message.id) > 0 {
					_, err = fmt.Fprintf(writer, "id: %s\n", message.id)
					handleError(err)
				}
				if len(message.event) > 0 {
					_, err = fmt.Fprintf(writer, "event: %s\n", message.event)
					handleError(err)
				}
				_, err = fmt.Fprintf(writer, "data: %s\n\n", message.data)
				handleError(err)
				flusher.Flush()
			case <-shutdownChannel:
			case <-request.Context().Done():
				sseConnection.isOpen = false
				return
			}
		}
	}()

	return sseConnection, nil
}
