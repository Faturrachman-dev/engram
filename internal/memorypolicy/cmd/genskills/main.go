// Command genskills regenerates every maintained artifact derived from the
// canonical memory policy: the shipped SKILL.md files and the generated
// ENGRAM_POLICY block inside the TypeScript plugins.
//
// Run from the repository root:
//
//	go run ./internal/memorypolicy/cmd/genskills
//
// or via `go generate ./internal/memorypolicy`.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Gentleman-Programming/engram/v3/internal/memorypolicy"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	for _, s := range memorypolicy.ShippedSkills {
		path := filepath.Join(*root, filepath.FromSlash(s.Path))
		if err := os.WriteFile(path, []byte(memorypolicy.Skill(s.Client)), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("wrote", s.Path)
	}

	for _, rel := range memorypolicy.TSPolicyFiles {
		path := filepath.Join(*root, filepath.FromSlash(rel))
		raw, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		out, ok := memorypolicy.ReplaceTSPolicyBlock(string(raw))
		if !ok {
			fail(fmt.Errorf("%s: generated memorypolicy markers not found", rel))
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("wrote", rel)
		if mirror, ok := memorypolicy.TSMirrors[rel]; ok {
			if err := os.WriteFile(filepath.Join(*root, filepath.FromSlash(mirror)), []byte(out), 0o644); err != nil {
				fail(err)
			}
			fmt.Println("wrote", mirror)
		}
	}

	for _, rel := range memorypolicy.MDPolicyFiles {
		path := filepath.Join(*root, filepath.FromSlash(rel))
		raw, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		out, ok := memorypolicy.ReplaceMDPolicyBlock(string(raw))
		if !ok {
			fail(fmt.Errorf("%s: generated memorypolicy markers not found", rel))
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("wrote", rel)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "genskills:", err)
	os.Exit(1)
}
