package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig_ParseURAMap(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		config := Config{}
		result := config.ParseURAMap()
		assert.Empty(t, result)
	})
	t.Run("1 entry", func(t *testing.T) {
		config := Config{URAMap: "ura1=did:example.com:bob"}
		result := config.ParseURAMap()
		assert.Len(t, result, 1)
		assert.Equal(t, "did:example.com:bob", result["ura1"])
	})
	t.Run("2 entries", func(t *testing.T) {
		config := Config{URAMap: "ura1=did:example.com:bob,ura2=did:example.com:alice"}
		result := config.ParseURAMap()
		assert.Len(t, result, 2)
		assert.Equal(t, "did:example.com:bob", result["ura1"])
		assert.Equal(t, "did:example.com:alice", result["ura2"])
	})
	t.Run("trims spaces", func(t *testing.T) {
		config := Config{URAMap: "ura1 = did:example.com:bob, ura2 = did:example.com:alice"}
		result := config.ParseURAMap()
		assert.Len(t, result, 2)
		assert.Equal(t, "did:example.com:bob", result["ura1"])
		assert.Equal(t, "did:example.com:alice", result["ura2"])
	})
}
