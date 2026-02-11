//go:build goexperiment.jsonv2

package internal

import (
	"encoding/json/jsontext"
	"io"
)

func MarshalJSON(out io.Writer, in any, indent string) error {
	var opts []json.Options
	if indent != "" {
		opts = make([]json.Options, 1)
		opts[0] = jsontext.WithIndent(indent)
	}

	return json.MarshalWrite(out, in, opts...)
}

func UnmarshalJSON(in io.Reader, out any) error {
	return json.UnmarshalRead(in, out)
}
