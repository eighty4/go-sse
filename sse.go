package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Connection struct {
	messages chan Message
	shutdown chan bool
	isOpen   bool
}

func (connection *Connection) BuildMessage() *MessageBuilder {
	return &MessageBuilder{&Message{}, connection}
}

func (connection *Connection) SendBytes(data []byte) error {
	return connection.BuildMessage().SendBytes(data)
}

func (connection *Connection) SendString(data string) error {
	return connection.BuildMessage().SendString(data)
}

func (connection *Connection) SendJson(data interface{}) error {
	return connection.BuildMessage().SendJson(data)
}

func (connection *Connection) IsOpen() bool {
	return connection.isOpen
}

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

type MessageBuilder struct {
	message    *Message
	connection *Connection
}

func (messageBuilder *MessageBuilder) WithId(id string) *MessageBuilder {
	messageBuilder.message.id = id
	return messageBuilder
}

func (messageBuilder *MessageBuilder) WithEvent(event string) *MessageBuilder {
	messageBuilder.message.event = event
	return messageBuilder
}

func (messageBuilder *MessageBuilder) SendBytes(data []byte) error {
	messageBuilder.message.data = data
	return messageBuilder.connection.send(messageBuilder.message)
}

func (messageBuilder *MessageBuilder) SendString(data string) error {
	messageBuilder.message.data = []byte(data)
	return messageBuilder.connection.send(messageBuilder.message)
}

func (messageBuilder *MessageBuilder) SendJson(data interface{}) error {
	if data, err := json.Marshal(data); err != nil {
		return err
	} else {
		messageBuilder.message.data = []byte(data)
		return messageBuilder.connection.send(messageBuilder.message)
	}
}

type Message struct {
	id    string
	event string
	data  []byte
}

func Upgrade(writer http.ResponseWriter, request *http.Request) (*Connection, error) {

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	sseConnection := &Connection{
		messages: make(chan Message),
		shutdown: make(chan bool),
		isOpen:   true,
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	go func() {
		for {
			select {
			case message := <-sseConnection.messages:
				if len(message.id) > 0 {
					fmt.Fprintf(writer, "id: %s\n", message.id)
				}
				if len(message.event) > 0 {
					fmt.Fprintf(writer, "event: %s\n", message.event)
				}
				fmt.Fprintf(writer, "data: %s\n\n", message.data)
				flusher.Flush()
			case <-sseConnection.shutdown:
			case <-request.Context().Done():
				sseConnection.isOpen = false
				return
			}
		}
	}()

	return sseConnection, nil
}
