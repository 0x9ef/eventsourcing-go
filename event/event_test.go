package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWithSerializerUndefined(t *testing.T) {
	_, err := NewWithSerializer("created", struct{}{}, "undefined")
	assert.EqualError(t, err, "unsupported serializer")
}
