package engine

import (
	"context"
	"time"
)

// ScheduleReply runs send now, or after delaySeconds via a detached timer.
//
// When delaySeconds <= 0 the send runs synchronously on the caller's context
// (immediate reply, current behaviour). When > 0 the send is deferred with a
// fresh background context, because the triggering webhook task / Long Poll
// event context is already done by the time the timer fires.
//
// Trade-off: the timer lives in-process, so a worker restart during the delay
// window drops that pending reply. Acceptable for short, human-like delays.
func ScheduleReply(ctx context.Context, delaySeconds int, send func(context.Context)) {
	if delaySeconds <= 0 {
		send(ctx)
		return
	}
	time.AfterFunc(time.Duration(delaySeconds)*time.Second, func() {
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		send(bg)
	})
}
