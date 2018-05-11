package kldexerciser

// Config is the Kaleido go-ethereum exerciser configuration
type KldExerciserConfig struct {
	URL        string
	Contract   string
	Txns       int
	Workers    int
	DebugLevel int
}
