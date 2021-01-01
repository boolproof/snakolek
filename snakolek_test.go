package main

import (
	"testing"
)

func TestAreCoordInSlice(t *testing.T) {
	head := coords{
		x: 5,
		y: 5,
	}

	body := []coords{head}
	result := areCoordsInSlice(head, body)

	if !result {
		t.Error("Coords expected to be in slice")
	}

	body = []coords{coords{3, 3}}
	result = areCoordsInSlice(head, body)

	if result {
		t.Error("Coords expected not to be in slice")
	}

	body = []coords{coords{3, 3}, coords{3, 4}, coords{3, 5}, coords{4, 5}}
	result = areCoordsInSlice(head, body)

	if result {
		t.Error("Coords expected not to be in slice")
	}

	head = coords{4, 5}
	result = areCoordsInSlice(head, body)

	if !result {
		t.Error("Coords expected to be in slice")
	}
}
