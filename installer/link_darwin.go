package main

// Wails' file dialogs use UTType, which lives in a framework Wails doesn't
// link itself; without this, go build and go test fail to link on macOS.

// #cgo LDFLAGS: -framework UniformTypeIdentifiers
import "C"
