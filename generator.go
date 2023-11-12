package eventsourcing

import (
	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

type IDGenerator func(alphabet string, size int) string

const (
	idDefaultAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	idDefaultSize     = 64
)

func UUIDGenerator(alphabet string, size int) string {
	return uuid.New().String()
}

func NanoidGenerator(alphabet string, size int) string {
	s, _ := gonanoid.Generate(alphabet, size)
	return s
}
