package internal

import (
	"errors"
	"fmt"
	"io"
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

	*alternativesBuilder
	variables map[string]string // name -> type.
}

type typeUnion struct {
	canonical   string
	definitions []*ast.Definition
}

func newTypeUnion(defs []*ast.Definition) typeUnion {
	names := make([]string, len(defs))
	for i, def := range defs {
		names[i] = StringToJSON(def.Name)
	}

	return typeUnion{
		definitions: defs,
		canonical:   canonicalizeUnion(names),
	}
}

func intersectUnions(a, b typeUnion) typeUnion {
	seen := make(map[string]bool)
	for _, def := range a.definitions {
		seen[def.Name] = true
	}
	var common []*ast.Definition
	for _, def := range b.definitions {
		if seen[def.Name] {
			common = append(common, def)
		}
	}
	return newTypeUnion(common)
}

// Produce a canonicalized TypeScript union, suitable for use as Go map keys.
func canonicalizeUnion(alternatives []string) string {
	if len(alternatives) == 0 {
		return "never"
	}
	sort.Strings(alternatives)
	return strings.Join(alternatives, " | ")
}

type alternativesBuilder struct {
	self         typeUnion                 // Current set of applicable concrete types.
	fields       map[string]string         // alias -> type.
	objects      map[string]*objectBuilder // concrete type name -> applicable
	alternatives map[string]typeUnion      // Set of possible type unions. Keyed by canonical.
}

type objectBuilder struct {
	fields    map[string]bool
	fragments map[string]bool
}

func newAlternativesBuilder(self typeUnion) *alternativesBuilder {
	b := &alternativesBuilder{
		self:    self,
		fields:  make(map[string]string),
		objects: make(map[string]*objectBuilder),
		alternatives: map[string]typeUnion{
			self.canonical: self,
		},
	}
	// The self constriant is only ever narrowed, so allocate builders for all
	// the possible concrete types.
	for _, def := range self.definitions {
		b.objects[def.Name] = &objectBuilder{
			fields:    make(map[string]bool),
			fragments: make(map[string]bool),
		}
	}
	return b
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

func (t *Typer) visitOperationDefinition(def *ast.OperationDefinition) string {
	var objectType *ast.Definition
	var opKind string
	switch def.Operation {
	case ast.Query:
		opKind = "Query"
		objectType = t.Schema.Query
	case ast.Mutation:
		opKind = "Mutation"
		objectType = t.Schema.Mutation
	case ast.Subscription:
		opKind = "Subscription"
		objectType = t.Schema.Subscription
	default:
		panic(fmt.Errorf("unexpected kind of operation: %q", def.Operation))
	}
	end := t.startDefinition(opKind, def.Name, objectType)
	t.visitVariableDefinitions(def.VariableDefinitions)
	t.visitSelectionSet(def.SelectionSet)
	return end()
}

func (t *Typer) toConcreteUnion(def *ast.Definition) typeUnion {
	switch def.Kind {
	case ast.Object:
		return newTypeUnion([]*ast.Definition{def})

	case ast.Interface:
		// TODO: This is be cacheable.
		var defs []*ast.Definition
		for _, candidate := range t.Schema.Types {
			if candidate.Kind != ast.Object {
				continue
			}
			for _, iface := range candidate.Interfaces {
				if iface == def.Name {
					defs = append(defs, candidate)
					break
				}
			}
		}
		return newTypeUnion(defs)

	case ast.Union:
		defs := make([]*ast.Definition, len(def.Types))
		for i, name := range def.Types {
			defs[i] = t.getDefinition(name)
		}
		return newTypeUnion(defs)

	case ast.Scalar, ast.Enum, ast.InputObject:
		panic(fmt.Errorf("expected only composite types, got %q", def.Kind))

	default:
		panic(fmt.Errorf("unknown kind: %q", def.Kind))
	}
}

func (t *Typer) visitFragmentDefinition(op *ast.FragmentDefinition) (documentType string) {
	objectType := t.getDefinition(op.TypeCondition)
	end := t.startDefinition("Fragment", op.Name, objectType)
	t.visitSelectionSet(op.SelectionSet)
	return end()
}

func (t *Typer) startDefinition(opKind, name string, objectType *ast.Definition) (end func() (documentType string)) {
	t.variables = make(map[string]string)
	endObject := t.startObject(objectType)
	return func() (documentType string) {
		dataType := endObject()
		documentType = t.buildDocumentType(opKind, name, dataType)
		t.variables = nil
		return
	}
}

func (t *Typer) startObject(typ *ast.Definition) (end func() (dataType string)) {
	oldBuilder := t.alternativesBuilder

	concreteTypes := t.toConcreteUnion(typ)
	t.alternativesBuilder = newAlternativesBuilder(concreteTypes)

	return func() string {
		dataType := t.buildDataType()
		t.alternativesBuilder = oldBuilder
		return dataType
	}
}

func (t *Typer) getDefinition(name string) *ast.Definition {
	return t.Schema.Types[name]
}

func (t *Typer) narrow(target *ast.Definition) (widen func()) {
	old := t.self
	u := intersectUnions(old, t.toConcreteUnion(target))
	t.self = u
	t.alternatives[u.canonical] = u
	return func() {
		t.self = old
	}
}

func (t *Typer) buildDocumentType(prefix, name, dataType string) (documentType string) {
	variablesType := t.buildVariablesType()

	if name != "" {
		t.Declarations = append(t.Declarations,
			fmt.Sprintf("export type %s_%s_Data = %s;", prefix, name, dataType),
			fmt.Sprintf("export type %s_%s_Variables = %s;", prefix, name, variablesType),
		)
		dataType = fmt.Sprintf("%s_%s_Data", prefix, name)
		variablesType = fmt.Sprintf("%s_%s_Variables", prefix, name)
	}

	return fmt.Sprintf("{ data: %s; variables: %s; }", dataType, variablesType)
}

func (t *Typer) buildDataType() string {
	if len(t.alternatives) == 0 {
		return "/* buildDataType */ never"
	}
	typenameUnions := make([]string, 0, len(t.alternatives))
	for key := range t.alternatives {
		typenameUnions = append(typenameUnions, key)
	}
	sort.Strings(typenameUnions)

	var b strings.Builder
	sep := ""
	for _, typenameUnion := range typenameUnions {
		b.WriteString(sep)
		sep = " | "
		t.writeObject(&b, t.alternatives[typenameUnion])
	}
	return b.String()
}

func (t *Typer) buildVariablesType() string {
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

func (t *Typer) writeObject(w io.Writer, types typeUnion) {
	fieldSet := make(map[string]bool)
	fragmentSet := make(map[string]bool)
	var fieldAliases, fragmentNames []string

	for _, def := range types.definitions {
		obj := t.objects[def.Name]
		for fieldAlias := range obj.fields {
			if fieldSet[fieldAlias] {
				continue
			}
			fieldSet[fieldAlias] = true
			fieldAliases = append(fieldAliases, fieldAlias)
		}
		for fragmentName := range obj.fragments {
			if fragmentSet[fragmentName] {
				continue
			}
			fragmentSet[fragmentName] = true
			fragmentNames = append(fragmentNames, fragmentName)
		}
	}
	sort.Strings(fieldAliases)
	sort.Strings(fragmentNames)

	fmt.Fprintf(w, "{ __typename: %s; ", types.canonical)
	for _, name := range fieldAliases {
		typ := t.fields[name]
		fmt.Fprintf(w, "%s: %s; ", name, typ)
	}
	fmt.Fprintf(w, "}")
	for _, name := range fragmentNames {
		fmt.Fprintf(w, " & Fragment_%s_Data", name)
	}
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
	t.variables[name] = t.visitType(def.Type)
}

func (t *Typer) visitSelectionSet(selections ast.SelectionSet) {
	for _, selection := range selections {
		t.visitSelection(selection)
	}
}

func (t *Typer) concreteTypename(name string) string {
	typ := t.Schema.Types[name]
	if typ != nil && typ.Kind == ast.Object {
		return StringToJSON(typ.Name)
	} else {
		return "string"
	}
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
	var fieldType string
	if node.SelectionSet == nil {
		fieldType = t.visitType(def.Type)
	} else {
		leafName, endType := t.beginType(def.Type)
		endObject := t.startObject(t.getDefinition(leafName))
		t.visitSelectionSet(node.SelectionSet)
		fieldType = endType(endObject())

	}
	t.fields[alias] = fieldType
	for _, def := range t.self.definitions {
		t.objects[def.Name].fields[alias] = true
	}
}

func (t *Typer) beginType(typ *ast.Type) (leafName string, end func(unwrapped string) (wrapped string)) {
	var stack []*ast.Type
	for {
		stack = append(stack, typ)
		if typ.Elem == nil {
			break
		}
		typ = typ.Elem
	}
	leafName = typ.NamedType
	end = func(unwrapped string) (wrapped string) {
		needsParens := strings.Contains(unwrapped, " ")
		var b strings.Builder
		for _, wrapper := range stack {
			if needsParens && !wrapper.NonNull || wrapper.Elem != nil {
				b.WriteString("(")
			}
		}
		b.WriteString(unwrapped)
		for i := len(stack) - 1; i >= 0; i-- {
			wrapper := stack[i]
			if needsParens {
				if !wrapper.NonNull || wrapper.Elem != nil {
					b.WriteString(")")
				}
				if wrapper.Elem != nil {
					b.WriteString("[]")
				}
			}
			if !wrapper.NonNull {
				b.WriteString(" | null")
				needsParens = true
			}
		}
		return b.String()
	}
	return
}

func (t *Typer) visitFragmentSpread(node *ast.FragmentSpread) {
	widen := t.narrow(t.getDefinition(node.Definition.TypeCondition))
	defer widen()

	if node.Name == "" {
		t.visitSelectionSet(node.Definition.SelectionSet)
	} else {
		for _, def := range t.self.definitions {
			t.objects[def.Name].fragments[node.Name] = true
		}
	}
}

func (t *Typer) visitInlineFragment(node *ast.InlineFragment) {
	widen := t.narrow(t.getDefinition(node.TypeCondition))
	defer widen()

	t.visitSelectionSet(node.SelectionSet)
}

func (t *Typer) visitType(typ *ast.Type) string {
	leafName, end := t.beginType(typ)
	switch leafName {
	case "String", "ID":
		leafName = "string"
	case "Boolean":
		leafName = "boolean"
	case "Int", "Float":
		leafName = "number"
	default:
		t.Scalars = append(t.Scalars, leafName)
	}
	return end(leafName)
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
