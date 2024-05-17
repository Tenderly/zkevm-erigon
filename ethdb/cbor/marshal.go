package cbor

import (
	"io"
)

func Marshal(dst io.Writer, v interface{}) error {
	return nil
}

func Unmarshal(dst interface{}, data io.Reader) error {
	return nil
}

func MustMarshal(dst io.Writer, v interface{}) {
	err := Marshal(dst, v)
	if err != nil {
		panic(err)
	}
}

func MustUnmarshal(dst interface{}, data io.Reader) {
	err := Unmarshal(dst, data)
	if err != nil {
		panic(err)
	}
}
