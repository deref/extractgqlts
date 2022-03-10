package internal

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type Typer struct {
	Schema *ast.Schema

	GeneratedTypes

	dataBuilder strings.Builder
	variables   map[string]string
}

type GeneratedTypes struct {
	QueryMap     []QueryType
	Declarations []string
}

type QueryType struct {
	Query string
	Type  string
}

func (t *Typer) VisitString(gql string) (string, error) {
	doc, gqlErrs := gqlparser.LoadQuery(t.Schema, gql)
	if len(gqlErrs) > 0 {
		return "", fmt.Errorf("loading query: %w", gqlErrs)
	}
	typ, err := t.visitDocument(doc)
	if err != nil {
		return "", err
	}
	t.GeneratedTypes.QueryMap = append(t.GeneratedTypes.QueryMap, QueryType{
		Query: gql,
		Type:  typ,
	})
	return typ, nil
}

func (t *Typer) visitDocument(doc *ast.QueryDocument) (string, error) {
	n := len(doc.Operations) + len(doc.Fragments)
	switch n {
	case 0:
		return "", errors.New("no definitions")
	case 1:
		for _, operation := range doc.Operations {
			typ := t.visitOperationDefinition(operation)
			return typ, nil
		}
		for _, fragment := range doc.Fragments {
			typ := t.visitFragmentDefinition(fragment)
			return typ, nil
		}
		panic("unreachable")
	default:
		// TODO: Support 1 operation + N fragments.
		return "", fmt.Errorf("expected exactly one definition, found %d", n)
	}
}

func (t *Typer) visitOperationDefinition(op *ast.OperationDefinition) string {
	t.dataBuilder.Reset()
	t.variables = make(map[string]string)

	t.visitVariableDefinitions(op.VariableDefinitions)
	t.visitSelectionSet(op.SelectionSet)

	data := t.dataBuilder.String()

	variableNames := make([]string, 0, len(t.variables))
	for variableName := range t.variables {
		variableNames = append(variableNames, variableName)
	}
	sort.Strings(variableNames)
	var variablesBuilder strings.Builder
	variablesBuilder.WriteString("{ ")
	for _, name := range variableNames {
		typ := t.variables[name]
		fmt.Fprintf(&variablesBuilder, "%s: %s; ", name, typ)
	}
	variablesBuilder.WriteString("}")
	variables := variablesBuilder.String()

	typ := fmt.Sprintf("{ data: %s; variables: %s; }", data, variables)

	if op.Name == "" {
		return typ
	}

	typeName := "Query_" + op.Name
	t.Declarations = append(t.Declarations, fmt.Sprintf("type %s = %s;", typeName, typ))
	return typeName
}

func (t *Typer) visitFragmentDefinition(op *ast.FragmentDefinition) string {
	panic("TODO: Fragments")
}

func (t *Typer) visitVariableDefinitions(vars ast.VariableDefinitionList) {
	for _, v := range vars {
		t.visitVariableDefinition(v)
	}
}

func (t *Typer) visitVariableDefinition(def *ast.VariableDefinition) {
	name := def.Variable
	if _, exists := t.variables[name]; exists {
		// TODO: Check for conflicts.
		return
	}
	t.variables[name] = t.visitTypeRef(def.Type)
}

func (t *Typer) visitSelectionSet(selections ast.SelectionSet) {
	t.dataBuilder.WriteString("{")
	for _, selection := range selections {
		t.visitSelection(selection)
	}
	t.dataBuilder.WriteString(" }")
}

func (t *Typer) visitSelection(node ast.Selection) {
	switch node := node.(type) {
	case *ast.Field:
		t.visitField(node)
	case *ast.FragmentSpread:
		t.visitFragmentSpread(node)
	case *ast.InlineFragment:
		t.visitInlineFragment(node)
	default:
		panic(fmt.Errorf("unexpected selection type: %T", node))
	}
}

func (t *Typer) visitField(node *ast.Field) {
	t.visitArgumentList(node.Arguments)
	def := node.Definition
	alias := node.Alias
	if alias == "" {
		alias = node.Name
	}
	t.dataBuilder.WriteString(" ")
	t.dataBuilder.WriteString(alias)
	t.dataBuilder.WriteString(": ")
	if node.SelectionSet == nil {
		t.dataBuilder.WriteString(t.visitTypeRef(def.Type))
	} else {
		t.visitSelectionSet(node.SelectionSet)
	}
	t.dataBuilder.WriteString(";")
}

func (t *Typer) visitFragmentSpread(node *ast.FragmentSpread) {
	panic("TODO: fragment spread")
}

func (t *Typer) visitInlineFragment(node *ast.InlineFragment) {
	panic("TODO: inline fragment")
}

func (t *Typer) visitTypeRef(typ *ast.Type) string {
	name := typ.Name()
	switch name {
	case "String":
		name = "string"
	case "Boolean":
		name = "boolean"
	default:
		/* no-op */
	}
	if !typ.NonNull {
		name = fmt.Sprintf("(%s | null)", name)
	}
	return name
}

func (t *Typer) visitArgumentList(args ast.ArgumentList) {
	for _, arg := range args {
		t.visitArgument(arg)
	}
}

func (t *Typer) visitArgument(arg *ast.Argument) {
	t.visitValue(arg.Value)
}

func (t *Typer) visitValue(v *ast.Value) {
	switch v.Kind {
	case ast.Variable:
		t.visitVariableDefinition(v.VariableDefinition)
	}
}
