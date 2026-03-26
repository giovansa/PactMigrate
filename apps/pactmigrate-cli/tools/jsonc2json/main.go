// jsonc2json converts a JSONC file (JSON with comments) to strict JSON for tools that only accept JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tidwall/jsonc"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: jsonc2json <input.jsonc> <output.json>")
		os.Exit(1)
	}
	in, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw := jsonc.ToJSON(in)
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Fprintln(os.Stderr, "jsonc parse:", err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out = append(out, '\n')
	if err := os.WriteFile(os.Args[2], out, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
