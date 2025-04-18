// Copyright 2025 The Kube Resource Orchestrator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package library

import (
	"crypto/sha256"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// RandomString returns a CEL function that generates deterministic random strings
// based on a seed.
//
// The function takes two arguments:
// - length: an integer specifying the length of the random string to generate
// - seed: a string used as the seed for the random string generation
//
// Example usage:
//
//	randomString(10, schema.spec.name)
//
// This will generate a random string of length 10 using the seed schema.spec.name.
// The same length and seed will always produce the same random string.

const (
	// alphanumericChars contains all possible characters for the random string
	alphanumericChars = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// RandomString returns a CEL function that generates deterministic random strings
func RandomString() cel.EnvOption {
	return cel.Function("randomString",
		cel.Overload("randomString_int_string",
			[]*cel.Type{cel.IntType, cel.StringType},
			cel.StringType,
			cel.BinaryBinding(generateDeterministicString),
		),
	)
}

// generateDeterministicString creates a deterministic random string based on a seed
func generateDeterministicString(length ref.Val, seed ref.Val) ref.Val {
	// Validate length
	lengthInt, ok := length.(types.Int)
	if !ok {
		return types.NewErr("randomString length must be an integer")
	}
	n := int(lengthInt.Value().(int64))
	if n <= 0 {
		return types.NewErr("randomString length must be positive")
	}

	// Validate seed
	seedStr, ok := seed.(types.String)
	if !ok {
		return types.NewErr("randomString seed must be a string")
	}

	// Generate hash from seed
	hash := sha256.Sum256([]byte(seedStr.Value().(string)))

	// Generate string from hash
	result := make([]byte, n)
	charsLen := len(alphanumericChars)
	for i := 0; i < n; i++ {
		// Use 4 bytes at a time from the hash
		start := (i * 4) % len(hash)
		end := start + 4
		if end > len(hash) {
			// If we run out of hash bytes, regenerate the hash with the current result
			newHash := sha256.Sum256(append(hash[:], result[:i]...))
			hash = newHash
			start = 0
			end = 4
		}
		// Convert 4 bytes to a uint32 and use it to select a character
		idx := uint32(hash[start])<<24 | uint32(hash[start+1])<<16 | uint32(hash[start+2])<<8 | uint32(hash[start+3])
		idx = idx % uint32(charsLen)
		result[i] = alphanumericChars[idx]
	}

	return types.String(string(result))
}
