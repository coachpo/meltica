package connection

import (
	"errors"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

func (r *sessionRuntime) enqueue(payload []byte) bool {
	for {
		select {
		case <-r.ctx.Done():
			return false
		default:
		}
		select {
		case r.incoming <- payload:
			return true
		default:
			if r.throttle <= 0 {
				return false
			}
			r.applyThrottle()
		}
	}
}

func (r *sessionRuntime) applyThrottle() {
	if r.throttle <= 0 {
		return
	}
	timer := time.NewTimer(r.throttle)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-r.ctx.Done():
	}
}

func (r *sessionRuntime) processLoop() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case payload, ok := <-r.incoming:
			if !ok {
				return
			}
			if len(payload) == 0 {
				continue
			}
			start := time.Now()
			env, err := r.decodePayload(payload)
			if err != nil {
				var metadata map[string]string
				if tracker := r.invalids; tracker != nil {
					count, thresholdExceeded := tracker.recordFailure()
					r.session.SetInvalidCount(count)
					metadata = map[string]string{
						"invalidCount": strconv.FormatUint(uint64(count), 10),
					}
					if thresholdExceeded {
						metadata["thresholdExceeded"] = "true"
					}
				} else {
					r.session.SetInvalidCount(0)
				}
				r.sendError(err)
				if r.metrics != nil {
					r.metrics.RecordFailure()
				}
				r.publish(telemetry.Event{Kind: telemetry.EventDecodeFailure, Error: err, Metadata: metadata})
				continue
			}
			if r.invalids != nil {
				r.invalids.reset()
			}
			r.session.SetInvalidCount(0)
			var summary dispatchSummary
			if r.dispatch != nil {
				summary = r.dispatch.Dispatch(r.ctx, env)
			}
			if r.metrics != nil {
				latency := summary.Latency
				if latency == 0 {
					latency = time.Since(start)
				}
				alloc := uint64(len(payload))
				errored := summary.Errors > 0
				r.metrics.RecordResult(latency, errored, alloc)
			}
			r.engine.pool.ReleaseEnvelope(env)
		}
	}
}

func (r *sessionRuntime) startHeartbeat() {
	if r.hbInterval <= 0 || r.hbTimeout <= 0 {
		return
	}
	ticker := time.NewTicker(r.hbInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				last := r.session.LastHeartbeat()
				if last.IsZero() {
					r.session.SetLastHeartbeat(time.Now().UTC())
				}
				if time.Since(last) > r.hbTimeout {
					r.shutdown(stream.SessionClosed, true, errors.New("heartbeat timeout"))
					return
				}
				_ = r.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
			}
		}
	}()
}
