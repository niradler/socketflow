package socketflow

import "log"

// Subscribe subscribes to a topic and returns a channel to receive messages
func (client *WebSocketClient) Subscribe(topic string) chan *Message {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[topic]; exists {
		return ch
	}

	ch := make(chan *Message, 10)
	client.subscriptions[topic] = ch
	return ch
}

// Unsubscribe unsubscribes from a topic
func (client *WebSocketClient) Unsubscribe(topic string) {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[topic]; exists {
		close(ch)
		delete(client.subscriptions, topic)
	}
}

// routeMessage routes a message to the appropriate topic channel
func (client *WebSocketClient) routeMessage(message Message) {
	client.subscribeMutex.Lock()
	defer client.subscribeMutex.Unlock()

	if ch, exists := client.subscriptions[message.Topic]; exists {
		ch <- &message
	} else {
		log.Printf("No subscribers for topic: %s\n", message.Topic)
	}
}
