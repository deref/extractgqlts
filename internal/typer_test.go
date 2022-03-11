package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestTyper(t *testing.T) {
	schema := gqlparser.MustLoadSchema(&ast.Source{
		Name: "schema.gql",
		Input: `
			type Query {
				hello: String!

				userById(id: String!): User
				currentUser: User

				now: Instant!

				named(name: String!): Named

				status: Status
			}

			scalar Instant

			interface Named {
				name: String!
			}

			type User implements Named {
				name: String!
				profile: String
			}

			type Pet implements Named {
				name: String!
				species: String!
			}

			union Status = Green | Red

			type Green {
				ok: Boolean!
			}

			type Red {
				ok: Boolean!
				message: String!
			}
		`,
	})
	// NOTE: These tests are not at all forgiving of whitespace, optional
	// semicolons, etc.  If the generated output conflicts with this, either make
	// the assertions less strict, or update the expected values to match.
	tests := []struct {
		Input                string
		ExpectedRoot         string
		ExpectedDeclarations GeneratedTypes
	}{
		{
			Input:        `{ hello }`,
			ExpectedRoot: `{ data: { hello: string; }; variables: { }; }`,
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
			ExpectedRoot: `{ data: Query_GetUser_Data; variables: Query_GetUser_Variables; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `query GetUser($userId: String!) { user: userById(id: $userId) { name, bio: profile } }`,
						Type:  `{ data: Query_GetUser_Data; variables: Query_GetUser_Variables; }`,
					},
				},
				Declarations: []string{
					`export type Query_GetUser_Data = { user: { name: string; bio: (string | null); }; };`,
					`export type Query_GetUser_Variables = { userId: string; };`,
				},
			},
		},
		{
			Input:        `fragment User on User { name, profile }`,
			ExpectedRoot: `{ data: Fragment_User_Data; variables: Fragment_User_Variables; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `fragment User on User { name, profile }`,
						Type:  `{ data: Fragment_User_Data; variables: Fragment_User_Variables; }`,
					},
				},
				Declarations: []string{
					`export type Fragment_User_Data = { name: string; profile: (string | null); };`,
					`export type Fragment_User_Variables = { };`,
				},
			},
		},
		{
			Input:        `query Clock { now }`,
			ExpectedRoot: `{ data: Query_Clock_Data; variables: Query_Clock_Variables; }`,
			ExpectedDeclarations: GeneratedTypes{
				Scalars: []string{
					"Instant",
				},
				QueryMap: []QueryType{
					{
						Query: `query Clock { now }`,
						Type:  `{ data: Query_Clock_Data; variables: Query_Clock_Variables; }`,
					},
				},
				Declarations: []string{
					`export type Query_Clock_Data = { now: Instant; };`,
					`export type Query_Clock_Variables = { };`,
				},
			},
		},
		//		{
		//			Input: `
		//query Fred { named(name: "fred") { ...Named, ... on Pet { species } } }
		//fragment Named on Named { name }
		//`,
		//			ExpectedType: `Query_Fred`,
		//			ExpectedDeclarations: GeneratedTypes{
		//				QueryMap: []QueryType{
		//					{
		//						Query: `
		//query Fred { named(name: "fred") { ...Named, ... on Pet { species } } }
		//fragment Named on Named { name }
		//`,
		//						Type: `Query_Fred`,
		//					},
		//				},
		//				Declarations: []string{
		//					`export type Fragment_Named = { data: { name: string; } & ({ __typename: string } | {__typename: "Dog", species: string }); variables: { }; };`,
		//					`export type Query_Fred = { data: { name: string; } & ({ __typename: string } | {__typename: "Dog", species: string }); variables: { }; };`,
		//				},
		//			},
		//		},
	}
	for _, test := range tests {
		typer := &Typer{
			Schema: schema,
		}
		actualRoot, err := typer.VisitString(test.Input)
		if !assert.NoError(t, err) {
			continue
		}
		if err != nil {
			panic(err)
		}
		actualDeclarations := typer.GeneratedTypes

		assert.Equal(t, test.ExpectedRoot, actualRoot)
		assert.Equal(t, test.ExpectedDeclarations, actualDeclarations)
	}
}
