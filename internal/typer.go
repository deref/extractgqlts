package internal

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
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

	errs := filterErrors(validator.Validate(t.Schema, doc))
	if len(errs) > 0 {
		return nil, errs
	}
	return doc, nil
}

func filterErrors(errs gqlerror.List) gqlerror.List {
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
	return errs
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
	switch len(doc.Operations) {
	case 0:
		switch len(doc.Fragments) {
		case 0:
			return "", errors.New("no definitions")
		case 1:
			typ := t.visitFragmentDefinition(doc.Fragments[0])
			return typ, nil
		default:
			return "", fmt.Errorf("expected at most one fragment definition, found %d", len(doc.Fragments))
		}
	case 1:
		for _, fragment := range doc.Fragments {
			t.visitFragmentDefinition(fragment)
		}
		typ := t.visitOperationDefinition(doc.Operations[0])
		return typ, nil
	default:
		return "", fmt.Errorf("expected at most one operation definition, found %d", len(doc.Operations))
	}
}

func (t *Typer) visitOperationDefinition(op *ast.OperationDefinition) string {
	t.reset()
	t.visitVariableDefinitions(op.VariableDefinitions)
	t.visitSelectionSet("Query", op.SelectionSet)
	return t.buildDefDocType("Query", op.Name)
}

func (t *Typer) visitFragmentDefinition(op *ast.FragmentDefinition) string {
	t.reset()
	t.visitSelectionSet(op.TypeCondition, op.SelectionSet)
	return t.buildDefDocType("Fragment", op.Name)
}

func (t *Typer) reset() {
	t.dataBuilder.Reset()
	t.variables = make(map[string]string)
}

func (t *Typer) buildDefDocType(prefix string, name string) string {
	data := t.dataBuilder.String()
	variables := t.buildVariables()

	if name != "" {
		t.Declarations = append(t.Declarations,
			fmt.Sprintf("export type %s_%s_Data = %s;", prefix, name, data),
			fmt.Sprintf("export type %s_%s_Variables = %s;", prefix, name, t.buildVariables()),
		)
		data = fmt.Sprintf("%s_%s_Data", prefix, name)
		variables = fmt.Sprintf("%s_%s_Variables", prefix, name)
	}

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

func (t *Typer) visitSelectionSet(typeCondition string, selections ast.SelectionSet) {
	t.dataBuilder.WriteString("{")

	typ := t.Schema.Types[typeCondition]
	if typ != nil && typ.Kind == ast.Object {
		fmt.Fprintf(&t.dataBuilder, " __typename: %s;", stringToJSON(typ.Name))
	}

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
		t.visitSelectionSet(def.Type.NamedType, node.SelectionSet)
	}
	t.dataBuilder.WriteString(";")
}

func (t *Typer) visitFragmentSpread(node *ast.FragmentSpread) {
	t.visitFragment(node.ObjectDefinition, node.Definition.SelectionSet)
}

func (t *Typer) visitInlineFragment(node *ast.InlineFragment) {
	t.visitFragment(node.ObjectDefinition, node.SelectionSet)
}

func (t *Typer) visitFragment(object *ast.Definition, selections ast.SelectionSet) {
	typ := object.Name
	t.dataBuilder.WriteString("} & ({ __typename: string } | { __typename: ")
	t.dataBuilder.WriteString(stringToJSON(typ))
	t.dataBuilder.WriteString("; ")
	t.visitSelectionSet(object.Name, selections)
	t.dataBuilder.WriteString("})")
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
