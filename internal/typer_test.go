package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func mustGenerateTypes(schema *ast.Schema, gql string) (string, GeneratedTypes) {
	t := &Typer{
		Schema: schema,
	}
	typ, err := t.VisitString(gql)
	if err != nil {
		panic(err)
	}
	return typ, t.GeneratedTypes
}

func TestTyper(t *testing.T) {
	schema := gqlparser.MustLoadSchema(&ast.Source{
		Name: "schema.gql",
		Input: `
			type Query {
				hello: String!
				
				userById(id: String!): User
				currentUser: User
				
				now: Instant!
			}
			
			type User {
				name: String!
				profile: String
			}
			
			scalar Instant
		`,
	})
	// NOTE: These tests are not at all forgiving of whitespace, optional
	// semicolons, etc.  If the generated output conflicts with this, either make
	// the assertions less strict, or update the expected values to match.
	tests := []struct {
		Input                string
		ExpectedType         string
		ExpectedDeclarations GeneratedTypes
	}{
		{
			Input:        `{ hello }`,
			ExpectedType: `{ data: { hello: string; }; variables: { }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `{ hello }`,
						Type:  `{ data: { hello: string; }; variables: { }; }`,
					},
				},
			},
		},
		{
			Input:        `query GetUser($userId: String!) { user: userById(id: $userId) { name, bio: profile } }`,
			ExpectedType: "Query_GetUser",
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `query GetUser($userId: String!) { user: userById(id: $userId) { name, bio: profile } }`,
						Type:  `Query_GetUser`,
					},
				},
				Declarations: []string{
					"type Query_GetUser = { data: { user: { name: string; bio: (string | null); }; }; variables: { userId: string; }; };",
				},
			},
		},
		{
			Input:        `fragment User on User { name, profile }`,
			ExpectedType: "Fragment_User",
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `fragment User on User { name, profile }`,
						Type:  `Fragment_User`,
					},
				},
				Declarations: []string{
					"type Fragment_User = { data: { name: string; profile: (string | null); }; variables: { }; };",
				},
			},
		},
		{
			Input:        `query Clock { now }`,
			ExpectedType: `Query_Clock`,
			ExpectedDeclarations: GeneratedTypes{
				Scalars: []string{
					"Instant",
				},
				QueryMap: []QueryType{
					{
						Query: `query Clock { now }`,
						Type:  `Query_Clock`,
					},
				},
				Declarations: []string{
					"type Query_Clock = { data: { now: Instant; }; variables: { }; };",
				},
			},
		},
	}
	for _, test := range tests {
		actualType, actualDeclarations := mustGenerateTypes(schema, test.Input)
		assert.Equal(t, test.ExpectedType, actualType)
		assert.Equal(t, test.ExpectedDeclarations, actualDeclarations)
	}
}
