package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"path/filepath"

	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/gqlerror"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

var schemaPath string

type stringFlags []string

func init() {
	flag.StringVar(&schemaPath, "schema", "", "path to graphql schema")
	flag.Parse()
}

func main() {
	g := &rootgen{}
	if err := g.run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if g.errors > 0 {
		os.Exit(1)
	}
}

type generator struct {
	schema *ast.Schema
	errors int
}

func (g *generator) warnf(message string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", v...)
	g.errors++
}

func (g *generator) run() error {
	flag.Parse()
	inputPatterns := flag.Args()
	if schemaPath == "" || len(inputPatterns) == 0 {
		return fmt.Errorf("usage: %s --schema=/path/to/schema.gql <input ...>", filepath.Base(os.Args[0]))
	}

	if err := g.loadSchema(); err != nil {
		return fmt.Errorf("loading schema: %w", err)
	}

	for _, inputPattern := range inputPatterns {
		inputPath := inputPattern // XXX glob
		g.visitInput(inputPath)
	}

	return nil
}

func (g *generator) loadSchema() (err error) {
	g.schema, err = loadSchema()
	return
}

func loadSchema() (*ast.Schema, error) {
	schemaBuf, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}

	return gqlparser.LoadSchema(&ast.Source{
		Name:  schemaPath,
		Input: schemaBuf,
	})
}

func (g *generator) loadQuery(src string) (*ast.QueryDocument, gqlerror.List) {
	return gqlparser.LoadQuery(g.schema, src)
}

func (g *generator) visitInput(inputPath string) {
	bs, err := ioutil.ReadFile(inputPath)
	if err != nil {
		g.warnf("reading %q: %w", inputPath, err)
		return
	}

	// XXX
}
