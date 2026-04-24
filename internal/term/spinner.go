package term

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const elapsedThreshold = 5 * time.Second

// Spinner shows a status line on stderr while a long-running operation is in
// progress. If stderr is not a terminal, all methods are silent no-ops.
type Spinner struct {
	label   string
	active  bool
	start   time.Time
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
	enabled bool
}

// NewSpinner creates a spinner that will display label on stderr.
// If stderr is not a TTY the spinner is inert.
func NewSpinner(label string) *Spinner {
	return &Spinner{
		label:   label,
		enabled: IsTerminal(os.Stderr),
	}
}

// Start begins displaying the spinner. Safe to call on an inert spinner.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled || s.active {
		return
	}

	s.active = true
	s.start = time.Now()
	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	fmt.Fprintf(os.Stderr, "%s...", s.label)

	go s.run()
}

// Stop clears the spinner line and releases the goroutine.
// Safe to call multiple times or on an inert spinner.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	close(s.stop)
	s.mu.Unlock()

	<-s.done
	fmt.Fprintf(os.Stderr, "\r\033[2K")
}

func (s *Spinner) run() {
	defer close(s.done)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			elapsed := time.Since(s.start).Truncate(time.Second)
			if elapsed >= elapsedThreshold {
				fmt.Fprintf(os.Stderr, "\r\033[2K%s... (%s)", s.label, elapsed)
			}
		}
	}
}
