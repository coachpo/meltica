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
			env, err := r.decoder.Decode(payload)
			if err != nil {
				r.sendError(err)
				if r.metrics != nil {
					r.metrics.RecordFailure()
				}
				r.publish(telemetry.Event{Kind: telemetry.EventDecodeFailure, Error: err})
				continue
			}
			if r.validator != nil {
				result := r.validator.Validate(env)
				r.session.SetInvalidCount(result.InvalidCount)
				if !result.Valid {
					if result.Err != nil {
						r.sendError(result.Err)
						metadata := map[string]string{
							"invalidCount": strconv.FormatUint(uint64(result.InvalidCount), 10),
						}
						if result.ThresholdExceeded {
							metadata["thresholdExceeded"] = "true"
						}
						r.publish(telemetry.Event{Kind: telemetry.EventValidationFailure, Error: result.Err, Metadata: metadata})
					}
					if r.metrics != nil {
						r.metrics.RecordFailure()
					}
					r.engine.pool.ReleaseEnvelope(env)
					continue
				}
			} else {
				r.session.SetInvalidCount(0)
				env.SetValidated(true)
			}
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
