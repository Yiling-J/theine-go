//go:build race
// +build race

package internal

func init() {
	raceDetector = true
}
