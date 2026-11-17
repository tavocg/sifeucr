//go:build ignore

package main

import "os"

func main() {
	data, err := os.ReadFile("../LICENSE")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("../handlers/static/LICENSE.txt", data, 0644); err != nil {
		panic(err)
	}
}
