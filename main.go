package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"path/filepath"

	"github.com/deref/gqltagts/internal"
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
	g := &generator{}
	if err := g.run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if g.errors > 0 {
		os.Exit(1)
	}
}

type generator struct {
	typer  internal.Typer
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
		inputPaths, err := filepath.Glob(inputPattern)
		if err != nil {
			g.warnf("error expanding filepath pattern %q: %w", inputPattern, err)
			continue
		}
		for _, inputPath := range inputPaths {
			g.visitInput(inputPath)
		}
	}

	fmt.Println("// GENERATED FILE. DO NOT EDIT.")
	fmt.Println()

	generated := g.typer.GeneratedTypes
	if len(generated.Scalars) > 0 {
		fmt.Print(`import {`)
		for _, scalar := range generated.Scalars {
			fmt.Print(" ")
			fmt.Print(scalar)
		}
		fmt.Println(` } from "./scalars.ts";`)
		fmt.Println()
	}

	if len(generated.Declarations) > 0 {
		for _, decl := range generated.Declarations {
			fmt.Println(decl)
		}
		fmt.Println()
	}

	fmt.Println("export type QueryTypes = {")
	for _, entry := range generated.QueryMap {
		fmt.Printf("  %s: %s;\n", internal.StringToJSON(entry.Query), entry.Type)
	}
	fmt.Println("}")

	return nil
}

func (g *generator) loadSchema() (err error) {
	g.typer.Schema, err = loadSchema()
	return
}

func loadSchema() (*ast.Schema, error) {
	schemaBuf, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}

	schema, gqlErr := gqlparser.LoadSchema(&ast.Source{
		Name:  schemaPath,
		Input: string(schemaBuf),
	})
	if gqlErr != nil {
		return nil, gqlErr
	}
	return schema, nil
}

func (g *generator) visitInput(inputPath string) {
	bs, err := ioutil.ReadFile(inputPath)
	if err != nil {
		g.warnf("reading %q: %w", inputPath, err)
		return
	}
	queries, err := internal.ExtractQueriesFromBytes(bs)
	if err != nil {
		g.warnf("extracting queries from %q: %w", inputPath, err)
		return
	}
	for _, query := range queries {
		g.typer.VisitString(query)
	}
}
