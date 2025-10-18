# Code Improvements Analysis

Analysis of potential improvements to the codebase based on modern library features.

## 1. Renamed `ProviderRaw` → `ParseFrame` ✅ IMPLEMENTED

### Problem
The name `ProviderRaw` was misleading. It suggested provider-specific raw data, but actually represents an **intermediate parsing frame** used across all exchanges.

### Solution
Renamed to `ParseFrame` with enhanced documentation (consistent with existing `WsFrame` naming):

```go
// ParseFrame captures provider-specific payloads prior to normalization.
// This is an intermediate structure used during parsing - raw exchange data
// before conversion to canonical events. Supports all exchange types.
// Complements WsFrame: WsFrame → Parser → ParseFrame → Event
type ParseFrame struct {
    returned   bool
    Provider   string
    StreamName string
    ReceivedAt int64
    Payload    json.RawMessage
}
```

### Usage
```go
// Before: Confusing name
raw, release, err := p.acquireProviderRaw(ctx)

// After: Clear intent + consistent naming
frame, release, err := p.acquireParseFrame(ctx)
```

### Pool Registration
```go
// ParseFrame (200 capacity):
//   - Frame during parsing phase, before canonicalization to Event
//   - Holds raw exchange-specific JSON during transformation
//   - Exchange-agnostic: Supports all provider types (Binance, Coinbase, etc.)
//   - Data flow: WsFrame → Parser → ParseFrame → Event
//   - Hot path: Temporary holder during JSON parsing and normalization
registerPool("ParseFrame", 200, func() interface{} { 
    return new(schema.ParseFrame) 
})
```

### Benefits
- ✅ **Consistent naming**: Matches existing `WsFrame` pattern
- ✅ **Clearer intent**: "Frame" indicates discrete data unit
- ✅ **Better documentation**: Shows data flow explicitly
- ✅ **Easier onboarding**: New developers see pattern immediately
- ✅ **Architectural clarity**: WsFrame → ParseFrame → Event progression

### Files Changed
- `internal/schema/provider.go` - Type definition
- `cmd/gateway/main.go` - Pool registration
- `internal/adapters/binance/parser.go` - Usage
- `internal/schema/pooled_types_test.go` - Tests

---

## 2. coder/websocket Event-Driven Support ❌ NOT APPLICABLE

### Analysis
After reviewing Context7 documentation, **coder/websocket is NOT event-driven**. 

The README shows examples from OTHER libraries (gobwas/ws, lesismal/nbio) that ARE event-driven, but coder/websocket deliberately avoids this pattern in favor of idiomatic Go.

### coder/websocket Philosophy
- ✅ Simple, minimal API
- ✅ Direct Read()/Write() calls with context
- ✅ No callbacks, no event handlers
- ✅ Idiomatic Go concurrency patterns

### Our Current Implementation (Already Optimal)
```go
// Current pattern (idiomatic coder/websocket)
for {
    msgType, data, err := conn.Read(ctx)  // ✅ Context-aware, clean
    if err != nil {
        // Handle error
    }
    // Process data
}
```

### Event-Driven Would Be MORE Complex
```go
// Event-driven pattern (NOT supported by coder/websocket)
conn.OnMessage(func(data []byte) {  // ❌ Not available
    // Would need:
    // - State management in callbacks
    // - Error handling in callbacks  
    // - Context propagation challenges
    // - Less predictable control flow
})
```

### Comparison: Event-Driven Libraries

| Library | Style | Our Use Case |
|---------|-------|--------------|
| **gobwas/ws** | Event-driven | ❌ Bloated, less idiomatic |
| **lesismal/nbio** | Event-driven | ❌ Complex, non-standard |
| **coder/websocket** | Direct calls | ✅ Simple, idiomatic, context-aware |

### Recommendation
**Keep current implementation.** Our usage of coder/websocket is already optimal and idiomatic.

**Why event-driven would be worse:**
1. More complex state management
2. Harder error handling
3. Less predictable control flow
4. Context cancellation challenges
5. Not supported by our chosen library anyway

---

## 3. sourcegraph/conc/stream for Ordered Processing ⚠️ POTENTIAL FUTURE USE

### Analysis
`conc/stream` provides **ordered stream processing** - tasks processed in parallel with serial callbacks maintaining order.

### Key Feature
```go
stream.New().WithMaxGoroutines(n)
```
- Processes tasks concurrently
- Callbacks execute serially in original order
- Useful for order-sensitive operations

### Context7 Documentation Example
```go
// stdlib approach (manual ordering)
func mapStream(in chan int, out chan int, f func(int) int) {
    tasks := make(chan func())
    taskResults := make(chan chan int)
    
    // Worker goroutines
    var workerWg sync.WaitGroup
    for i := 0; i < 10; i++ {
        workerWg.Add(1)
        go func() {
            defer workerWg.Done()
            for task := range tasks {
                task()
            }
        }()
    }
    
    // Ordered reader goroutines
    var readerWg sync.WaitGroup
    readerWg.Add(1)
    go func() {
        defer readerWg.Done()
        for result := range taskResults {
            item := <-result
            out <- item
        }
    }()
    
    // Feed workers
    for elem := range in {
        resultCh := make(chan int, 1)
        taskResults <- resultCh
        tasks <- func() {
            resultCh <- f(elem)
        }
    }
    
    close(tasks)
    workerWg.Wait()
    close(taskResults)
    readerWg.Wait()
}

// conc/stream approach (clean)
func mapStream(in chan int, out chan int, f func(int) int) {
    s := stream.New().WithMaxGoroutines(10)
    for elem := range in {
        elem := elem
        s.Go(func() stream.Callback {
            res := f(elem)
            return func() { out <- res }  // Serial callback
        })
    }
    s.Wait()
}
```

### Where It Could Help

#### 1. BookAssembler Delta Processing (Order-Sensitive)
```go
// Current: Manual ordering via channels
type BookAssembler struct {
    updates chan *depthUpdate
    // Must process in sequence order
}

// Potential with conc/stream
s := stream.New().WithMaxGoroutines(1)  // Serial for ordering
for update := range updates {
    update := update
    s.Go(func() stream.Callback {
        processed := applyDelta(update)
        return func() { publishSnapshot(processed) }
    })
}
```

**Analysis:** Our current channel-based approach is simpler. conc/stream adds overhead without benefit for this use case.

#### 2. Event Fan-In (Our Current Implementation)
```go
// Current: Simple fan-in (ORDER NOT REQUIRED)
for i, evtChan := range eventChannels {
    go func(ch <-chan *schema.Event) {
        for evt := range ch {
            mergedEvents <- evt
        }
    }(evtChan)
}

// With conc/stream (unnecessary complexity)
s := stream.New().WithMaxGoroutines(len(eventChannels))
for _, evtChan := range eventChannels {
    evtChan := evtChan
    s.Go(func() stream.Callback {
        evt := <-evtChan
        return func() { mergedEvents <- evt }  // Serial callback
    })
}
```

**Analysis:** Events from different providers are **independent** - no ordering needed. Dispatcher handles time-based ordering later. Current implementation is simpler and faster.

### Recommendation
**Don't use conc/stream for now.** Our use cases either:
1. Don't need ordering (fan-in)
2. Are simple enough without it (BookAssembler)

### Potential Future Use Cases
- **Cross-exchange arbitrage**: Need to process price updates in order
- **Backtesting**: Replay historical events in exact order
- **Audit logging**: Maintain strict event sequencing

### When to Consider conc/stream
✅ When you have:
- Ordered input stream
- Need parallel processing
- Must maintain output order
- Complex ordering logic

❌ Don't use when:
- Simple channel fan-in suffices
- Order doesn't matter
- Single goroutine is fast enough

---

## Summary

| Improvement | Status | Reason |
|-------------|--------|--------|
| **Rename ProviderRaw → ParseFrame** | ✅ **DONE** | Consistent with WsFrame, clearer intent |
| **Event-driven WebSocket** | ❌ **N/A** | Not supported by coder/websocket; current approach is optimal |
| **conc/stream ordered processing** | ⚠️ **DEFER** | Not needed for current use cases; keep for future |

---

## Architectural Principles Maintained

### 1. **Simplicity Over Complexity**
- Chose idiomatic Go patterns
- Avoided unnecessary abstractions
- Used stdlib+minimal libs where possible

### 2. **Context-First Design**
- All operations respect context cancellation
- Proper timeout handling
- Clean shutdown semantics

### 3. **Memory Efficiency**
- Object pools for hot paths
- Zero-copy where possible
- Controlled allocations

### 4. **Extensibility**
- Exchange-agnostic abstractions (ParserBuffer)
- Multi-provider architecture
- Clean adapter pattern

---

## Dependencies Review

### Current Dependencies (Minimal & Modern)
```go
github.com/coder/websocket v1.8.14     // ✅ Modern WebSocket
github.com/goccy/go-json                // ✅ Fast JSON
github.com/sourcegraph/conc             // ✅ Structured concurrency
```

### Removed Dependencies
```diff
- github.com/gorilla/websocket  // ❌ Deprecated
```

### Not Added (Intentionally)
```
// Event-driven WebSocket libs
- gobwas/ws         // ❌ Bloated
- lesismal/nbio     // ❌ Complex
```

---

## Code Quality Metrics

### Before Improvements
```
- ProviderRaw: Confusing name
- gorilla/websocket: Deprecated
- Manual pool comments: Minimal
```

### After Improvements
```
✅ ParserBuffer: Clear, descriptive
✅ coder/websocket v1.8.14: Modern, maintained
✅ Enhanced documentation: Comprehensive pool comments
✅ All tests passing: 100% green
```

### Build Status
```bash
$ go build ./...
✅ Success

$ go test ./...
✅ ok  	github.com/coachpo/meltica/internal/schema
✅ ok  	github.com/coachpo/meltica/internal/adapters/binance
✅ ok  	github.com/coachpo/meltica/internal/pool
```

---

## Future Considerations

### When to Use conc/stream
1. **Backtesting engine**: Replay historical events in order
2. **Audit log processor**: Maintain strict sequencing
3. **Multi-exchange router**: Order-sensitive arbitrage

### Example Future Use
```go
// Backtesting: Process historical events in order
func replayHistory(events []HistoricalEvent) {
    s := stream.New().WithMaxGoroutines(10)
    for _, evt := range events {
        evt := evt
        s.Go(func() stream.Callback {
            // Process in parallel
            result := processEvent(evt)
            // But output in order
            return func() { publishResult(result) }
        })
    }
    s.Wait()
}
```

---

## References

### Documentation
- [coder/websocket README](https://github.com/coder/websocket/blob/master/README.md)
- [sourcegraph/conc README](https://github.com/sourcegraph/conc/blob/main/README.md)
- Context7 library documentation

### Related Docs
- `docs/multi-provider-architecture.md` - Multi-provider design
- `internal/adapters/binance/README.md` - Binance adapter
- `docs/binance-production-setup.md` - Production setup

---

**Conclusion:** We've made pragmatic improvements based on actual needs, not just "because new library exists". The codebase is cleaner, better documented, and uses modern, maintained libraries without unnecessary complexity.
