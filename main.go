package main

import (
	"./server"
)

// TODO: Document
// TODO: Release
// TODO: Add to Travis
// TODO: Reorder Dockerfile instructions
// TODO: Integration tests
// TODO: Alert snippets
// TODO: Alert labels
// TODO: Alert annotations
func main() {
	server.New().Execute()
}
