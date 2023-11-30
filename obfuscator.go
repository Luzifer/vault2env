package main

import (
	"crypto/sha256"
	"fmt"
)

func replaceAsterisk(_, _ string) string { return "****" }
func replaceHash(_, secret string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(secret)))
}
func replaceName(name, _ string) string { return name }

func getReplaceFn(name string) replaceFn {
	switch name {
	case "asterisk":
		return replaceAsterisk

	case "hash":
		return replaceHash

	case "name":
		return replaceName

	default:
		return nil
	}
}
