package main

import (
	"fmt"
	"testing"
)

func TestParseValue(t *testing.T) {
	slot := ParseSlot(
		[]interface{}{
			1.0,
			"景点",
			"周边景点",
			[]interface{}{},
			false,
		},
		"testDialogue",
		0)
	fmt.Printf("%#v\n", slot)
}
