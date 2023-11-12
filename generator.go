package eventsourcing

import (
	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

// IDGenerator is a function that generates random a string.
// Uses passed alphabet to generate a string only from provided characters and for predefined size.
type IDGenerator func(alphabet string, size int) string

const (
	idDefaultAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	idDefaultSize     = 64
)

func UUIDGenerator(_ string, _ int) string {
	return uuid.New().String()
}

func NanoidGenerator(alphabet string, size int) string {
	s, _ := gonanoid.Generate(alphabet, size)
	return s
}
