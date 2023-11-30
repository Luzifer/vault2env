package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

type (
	obfuscator struct {
		buffer             []byte
		closed             bool
		longestSecretLen   int
		output             io.Writer
		secretReplacements [][2]string
	}

	replaceFn func(name, secret string) string
)

var _ io.WriteCloser = &obfuscator{}

func newObfuscator(output io.Writer, secrets map[string]string, fn replaceFn) *obfuscator {
	// We're looking for the longest secret: That is half the amount of
	// data we need to keep in the buffer in order to detect and replace
	// the secrets before forwarding the data to the real writer
	var longestSecretLen int
	for _, s := range secrets {
		if l := len(s); l > longestSecretLen {
			longestSecretLen = l
		}
	}

	var replacements [][2]string
	if fn == nil {
		// Special case: No replacer is set, we can pass-through
		longestSecretLen = 0
	} else {
		// Create a map of replacements
		for name, secret := range secrets {
			replacements = append(replacements, [2]string{secret, fn(name, secret)})
		}
	}

	sort.Slice(replacements, func(j, i int) bool {
		return len(replacements[i][0]) < len(replacements[j][0])
	})

	return &obfuscator{
		longestSecretLen:   longestSecretLen,
		output:             output,
		secretReplacements: replacements,
	}
}

func (o *obfuscator) Close() (err error) {
	o.closed = true

	// Do a last sweep on the remaining buffer
	o.sanitizeBuffer()

	// Copy the rest to the underlying writer
	if _, err = o.output.Write(o.buffer); err != nil {
		return fmt.Errorf("writing remaining buffer: %w", err)
	}

	return nil
}

func (o *obfuscator) Write(data []byte) (n int, err error) {
	if o.closed {
		return 0, fmt.Errorf("write on closed writer")
	}

	// First take everything from the input
	o.buffer = append(o.buffer, data...)

	// If we haven't enough data buffered lets just pretent we wrote
	// everything and in reality do nothing
	if len(o.buffer) < o.longestSecretLen*2 {
		return len(data), nil
	}

	// Now we have at least twice the length of the longest secret in
	// the buffer so we can sanitize the buffer…
	o.sanitizeBuffer()

	// Now that all secrets have been replaced, we can write everything
	// to the writer except the last {longestSecretLen} bytes as they
	// might contain a part of the longest secret
	wrLen := len(o.buffer) - o.longestSecretLen
	if wrLen < 1 {
		// Nothing to write, buffer was shortened too much
		return len(data), nil
	}

	if _, err = io.Copy(o.output, bytes.NewReader(o.buffer[:wrLen])); err != nil {
		return 0, fmt.Errorf("copying sanitized data to writer: %w", err)
	}

	o.buffer = o.buffer[wrLen:]

	// We took everything from them: Lets tell them we wrote everything
	return len(data), nil
}

func (o *obfuscator) sanitizeBuffer() {
	for _, repl := range o.secretReplacements {
		o.buffer = bytes.ReplaceAll(o.buffer, []byte(repl[0]), []byte(repl[1]))
	}
}
