package eventsourcing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUUIDGenerator(t *testing.T) {
	x := UUIDGenerator("", 0) // always 36 symbols
	assert.Equal(t, 36, len(x))
}

func TestNanoidGenerator(t *testing.T) {
	x := NanoidGenerator("abc", 32) // variable length of output string
	assert.Equal(t, 32, len(x))
}
