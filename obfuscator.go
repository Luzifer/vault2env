package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	log "github.com/sirupsen/logrus"
)

func prepareObfuscator(secrets map[string]string) func(string) string {
	var prepare func(name, secret string) string

	switch cfg.Obfuscate {
	case "asterisk":
		prepare = func(name, secret string) string { return "****" }

	case "hash":
		prepare = func(name, secret string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(secret))) }

	case "name":
		prepare = func(name, secret string) string { return name }

	default:
		return func(in string) string { return in }
	}

	replacements := []string{}

	for k, v := range secrets {
		if k != "" && v != "" {
			replacements = append(replacements, v, prepare(k, v))
		}
	}
	repl := strings.NewReplacer(replacements...)

	return func(in string) string { return repl.Replace(in) }
}

func obfuscationTransport(in io.Reader, out io.Writer, obfuscate func(string) string) {
	s := bufio.NewScanner(bufio.NewReader(in))
	for s.Scan() {
		fmt.Fprintln(out, obfuscate(s.Text()))
	}
	if err := s.Err(); err != nil {
		log.WithError(err).Error("Failed to scan in buffer")
	}
}
