package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObfuscation(t *testing.T) {
	cases := []struct {
		Input     string
		Expected  string
		ReplaceFn replaceFn
	}{
		{
			Input:     "this is a longer string with a secret embedded inside",
			Expected:  "this is a longer string with a **** embedded inside",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "this is very short",
			Expected:  "this is very short",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "secret",
			Expected:  "****",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "foo",
			Expected:  "foo",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "can we have this very long secret with some special #$% chars in it obfuscated?",
			Expected:  "can we have **** obfuscated?",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "secretsecret",
			Expected:  "********",
			ReplaceFn: replaceAsterisk,
		},
		{
			Input:     "secret",
			Expected:  "secret",
			ReplaceFn: nil, // Direct pass-through
		},
	}

	for _, c := range cases {
		t.Run(c.Input, func(t *testing.T) {
			out := new(bytes.Buffer)
			obf := newObfuscator(out, map[string]string{
				"mysecret":   "secret",
				"longsecret": "this very long secret with some special #$% chars in it",
			}, c.ReplaceFn)

			_, err := fmt.Fprint(obf, c.Input)
			require.NoError(t, err)

			require.NoError(t, obf.Close())

			assert.Equal(t, c.Expected, out.String())
		})
	}
}
