package core

// Centralized custom primitive types used across the codebase.
// Market enumerates the product families supported by the protocol.
type Market string

// Topic is the concrete websocket topic identifier (e.g., "book:BTC-USDT").
type Topic string

// TopicTemplate is the channel portion before the colon (e.g., "book").
type TopicTemplate string

// Event is the canonical websocket event identifier.
type Event string

// TopicNone is the canonical empty topic value.
const TopicNone Topic = ""
