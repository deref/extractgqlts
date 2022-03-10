package internal

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

type Typer struct {
	Schema *ast.Schema

	GeneratedTypes

	dataBuilder strings.Builder
	variables   map[string]string
}

type GeneratedTypes struct {
	Scalars      []string
	QueryMap     []QueryType
	Declarations []string
}

type QueryType struct {
	Query string
	Type  string
}

func (t *Typer) loadQuery(gql string) (*ast.QueryDocument, error) {
	doc, err := parser.ParseQuery(&ast.Source{Input: gql})
	if err != nil {
		return nil, err
	}

	errs := validator.Validate(t.Schema, doc)
	dst := 0
	for i := 0; i < len(errs); i++ {
		err := errs[i]
		if ignoreError(err) {
			continue
		}
		errs[dst] = err
		dst++
	}
	errs = errs[:dst]
	if len(errs) > 0 {
		return nil, errs
	}
	return doc, nil
}

func ignoreError(err error) bool {
	switch {
	case strings.HasSuffix(err.Error(), "is never used."):
		return true
	default:
		return false
	}
}

func (t *Typer) VisitString(gql string) (string, error) {
	doc, err := t.loadQuery(gql)
	if err != nil {
		return "", fmt.Errorf("loading query: %w", err)
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
	t.reset()
	t.visitVariableDefinitions(op.VariableDefinitions)
	t.visitSelectionSet(op.SelectionSet)
	return t.buildDefDocType("Query", op.Name)
}

func (t *Typer) visitFragmentDefinition(op *ast.FragmentDefinition) string {
	t.reset()
	t.visitSelectionSet(op.SelectionSet)
	return t.buildDefDocType("Fragment", op.Name)
}

func (t *Typer) reset() {
	t.dataBuilder.Reset()
	t.variables = make(map[string]string)
}

func (t *Typer) buildDefDocType(prefix string, name string) string {
	typ := t.buildDocType()

	if name == "" {
		return typ
	}

	typeName := prefix + "_" + name
	t.Declarations = append(t.Declarations, fmt.Sprintf("type %s = %s;", typeName, typ))
	return typeName
}

func (t *Typer) buildDocType() string {
	data := t.dataBuilder.String()
	variables := t.buildVariables()
	return fmt.Sprintf("{ data: %s; variables: %s; }", data, variables)
}

func (t *Typer) buildVariables() string {
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
	return variablesBuilder.String()
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
	case "String", "ID":
		name = "string"
	case "Boolean":
		name = "boolean"
	case "Int", "Float":
		name = "number"
	default:
		t.Scalars = append(t.Scalars, name)
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
