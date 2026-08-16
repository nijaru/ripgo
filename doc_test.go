package ripgo_test

import (
	"context"
	"fmt"
	"os"

	"github.com/nijaru/ripgo"
	"github.com/nijaru/ripgo/find"
	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/printer"
	"github.com/nijaru/ripgo/search"
)

func ExampleSearch() {
	ctx := context.Background()

	for res, err := range ripgo.Search(ctx, `^package ripgo`, []string{"doc.go"},
		ripgo.WithTypes([]string{"go"}),
	) {
		if err != nil {
			continue
		}
		for _, m := range res.Matches {
			fmt.Printf("%s:%d:%d: %s\n", res.Path, m.Line, m.Column, m.LineBytes)
		}
	}
	// Output:
	// doc.go:60:1: package ripgo
}

func ExampleFind() {
	ctx := context.Background()

	for result, err := range ripgo.Find(ctx, `^doc\.go$`, []string{"doc.go"},
		ripgo.WithFindType(find.TypeFile),
		ripgo.WithFindExtension("go"),
	) {
		if err != nil {
			continue
		}
		fmt.Println(result.Path)
	}
	// Output:
	// doc.go
}

func ExampleSearcher() {
	m, _ := pattern.New(pattern.Config{Pattern: `^package ripgo`})
	s := search.NewSearcher(nil, search.Config{MaxCount: 1}, m)
	result, _ := s.SearchPath("doc.go", nil)

	p := printer.NewTextPrinter(printer.TextConfig{
		Writer:     os.Stdout,
		LineNumber: true,
	})
	_ = p.PrintResult(result)
	// Output:
	// doc.go:60:package ripgo
}
