//go:build test_unit || test_integration

package player

// SetStreamGen sets the stream generation, for tests of code that tells
// events of a replaced stream from the current one's.
func (p *Player) SetStreamGen(gen uint64) { p.streamGen.Store(gen) }
