//go:build !darwin && !windows && !linux

package main

import "log"

func main() {
	log.Fatal("showcase requires a supported native desktop backend")
}
