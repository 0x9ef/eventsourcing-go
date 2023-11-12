package event

import (
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
)

type Serializer interface {
	Encode(v interface{}) (Payload, error)
	Decode(data Payload, dst interface{}) error
}

type SerializerType string

const (
	SerializerTypeJSON SerializerType = "json"
	SerializerTypeBSON SerializerType = "bson"
)

// MatchedSerializers represents all currently available serializers.
var MatchedSerializers = map[SerializerType]Serializer{
	SerializerTypeJSON: &JSONSerializer{},
	SerializerTypeBSON: &BSONSerializer{},
}

type JSONSerializer struct{}

func (JSONSerializer) Encode(v interface{}) (Payload, error) {
	return json.Marshal(v)
}

func (JSONSerializer) Decode(data Payload, dst interface{}) error {
	return json.Unmarshal(data, &dst)
}

type BSONSerializer struct{}

func (BSONSerializer) Encode(v interface{}) (Payload, error) {
	return bson.Marshal(v)
}

func (BSONSerializer) Decode(data Payload, dst interface{}) error {
	return bson.Unmarshal(data, &dst)
}

type UnsupportedSerializer struct{}

func (UnsupportedSerializer) Encode(v interface{}) (Payload, error) {
	return nil, errors.New("unsupported serializer")
}

func (UnsupportedSerializer) Decode(data Payload, dst interface{}) error {
	return errors.New("unsupported serializer")
}
