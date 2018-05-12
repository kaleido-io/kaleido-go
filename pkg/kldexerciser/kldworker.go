package kldexerciser

import (
	log "github.com/sirupsen/logrus"
)

// Worker runs the specified number transactions the specified number of times then exits
type Worker struct {
	Name             string
	Exerciser        *Exerciser
	CompiledContract *CompiledSolidity
}

func (w Worker) debug(message string) {
	log.Debug(w.Name, ": ", message)
}

// Run executes the specified exerciser workload then exits
func (w Worker) Run() {
	w.debug("started")

	w.debug("finished")
}
