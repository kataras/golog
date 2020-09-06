package golog

import (
	"encoding/json"
	"sync"
)

// JSON returns a new JSON handler.
// The logger will print the logs in JSON format.
//
// Usage:
// logger.Handle(JSON("    "))
func JSON(indent string) Handler {
	// use one encoder per level, do not create new each time.
	encoders := make(map[Level]*json.Encoder, len(Levels))
	mu := new(sync.RWMutex)  // encoders locker.
	encMu := new(sync.Mutex) // encode action locker.

	return func(l *Log) bool {
		mu.RLock()
		enc, ok := encoders[l.Level]
		mu.RUnlock()

		if !ok {
			enc = json.NewEncoder(l.Logger.getLevelOutput(l.Level))
			enc.SetIndent("", indent)
			mu.Lock()
			encoders[l.Level] = enc
			mu.Unlock()
		}

		encMu.Lock()
		err := enc.Encode(l)
		encMu.Unlock()
		return err == nil
	}
}
