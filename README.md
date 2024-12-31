# SocketFlow

[![Go Reference](https://pkg.go.dev/badge/github.com/niradler/socketflow.svg)](https://pkg.go.dev/github.com/niradler/socketflow)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/niradler/socketflow/blob/main/LICENSE)

SocketFlow is a lightweight, efficient, and easy-to-use WebSocket library for Go. It supports **topic-based messaging**, **automatic chunking for large payloads**, and **real-time two-way communication**. Designed with simplicity and performance in mind, SocketFlow is perfect for building real-time applications like chat systems, live updates, and more.

## Features

- **Topic-Based Messaging**: Send and receive messages based on topics.
- **Automatic Chunking**: Automatically splits large payloads into chunks and reassembles them on the receiving end.
- **Real-Time Two-Way Communication**: Supports both request-response and pub/sub patterns.
- **Thread-Safe**: Uses mutexes to ensure thread safety.
- **Easy-to-Use API**: Simple and intuitive API for sending, receiving, and subscribing to messages.

## Installation

```bash
go get github.com/niradler/socketflow