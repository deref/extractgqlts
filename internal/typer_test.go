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
				allUsers: [User!]!

				now: Instant!

				named(name: String!): Named!

				status: Status

				concatAll(stringLists: [[String]]): String!
				sum(ints: [Int!]): Int!
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
	// NOTE: These are essentially gold-file tests and therefore are brittle.
	tests := []struct {
		Input                string
		ExpectedRoot         string
		ExpectError          bool
		ExpectedDeclarations GeneratedTypes
	}{
		// Simplest declaration.
		{
			Input:        `{ hello }`,
			ExpectedRoot: `{ data: { __typename: "Query"; hello: string; }; variables: { }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `{ hello }`,
						Type:  `{ data: { __typename: "Query"; hello: string; }; variables: { }; }`,
					},
				},
			},
		},
		// Variables, aliases, optionals.
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
					`export type Query_GetUser_Data = { __typename: "Query"; user: (({ __typename: "User"; bio: (string | null); name: string; }) | null); };`,
					`export type Query_GetUser_Variables = { userId: string; };`,
				},
			},
		},
		// Lists.
		{
			Input:        `{ allUsers { name } }`,
			ExpectedRoot: `{ data: { __typename: "Query"; allUsers: ({ __typename: "User"; name: string; })[]; }; variables: { }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `{ allUsers { name } }`,
						Type:  `{ data: { __typename: "Query"; allUsers: ({ __typename: "User"; name: string; })[]; }; variables: { }; }`,
					},
				},
			},
		},
		// Fragment declaration.
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
					`export type Fragment_User_Data = { __typename: "User"; name: string; profile: (string | null); };`,
					`export type Fragment_User_Variables = { };`,
				},
			},
		},
		// Custom scalar.
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
					`export type Query_Clock_Data = { __typename: "Query"; now: Instant; };`,
					`export type Query_Clock_Variables = { };`,
				},
			},
		},
		// Named and anonymous fragment spreads.
		{
			Input: `
query Fred { named(name: "fred") { ...Named, ... on Pet { species } } }
fragment Named on Named { name }
`,
			ExpectedRoot: `{ data: Query_Fred_Data; variables: Query_Fred_Variables; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `
query Fred { named(name: "fred") { ...Named, ... on Pet { species } } }
fragment Named on Named { name }
`,
						Type: `{ data: Query_Fred_Data; variables: Query_Fred_Variables; }`,
					},
				},
				Declarations: []string{
					`export type Fragment_Named_Data = { __typename: "Pet" | "User"; name: string; };`,
					`export type Fragment_Named_Variables = { };`,
					// TODO: Is it possible to enhance the algorith to simplify this?
					// Is it worth it? TypeScript should handle that for us.
					`export type Query_Fred_Data = { __typename: "Query"; named: ({ __typename: "Pet"; species: string; } & Fragment_Named_Data | { __typename: "Pet" | "User"; species: string; } & Fragment_Named_Data); };`,
					`export type Query_Fred_Variables = { };`,
				},
			},
		},
		// Nested lists with nullability.
		{
			Input:        `query ($stringLists: [[String]]) { concatAll(stringLists: $stringLists) }`,
			ExpectedRoot: `{ data: { __typename: "Query"; concatAll: string; }; variables: { stringLists: (((string | null)[] | null)[] | null); }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `query ($stringLists: [[String]]) { concatAll(stringLists: $stringLists) }`,
						Type:  `{ data: { __typename: "Query"; concatAll: string; }; variables: { stringLists: (((string | null)[] | null)[] | null); }; }`,
					},
				},
			},
		},
		// Nullable list with non-null elements.
		{
			Input:        `query ($ints: [Int!]) { sum(ints: $ints) }`,
			ExpectedRoot: `{ data: { __typename: "Query"; sum: number; }; variables: { ints: (number[] | null); }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `query ($ints: [Int!]) { sum(ints: $ints) }`,
						Type:  `{ data: { __typename: "Query"; sum: number; }; variables: { ints: (number[] | null); }; }`,
					},
				},
			},
		},
		// Errors.
		{
			Input:        `{`,
			ExpectedRoot: `unknown /* ERROR: input:1: Expected Name, found <EOF> */`,
			ExpectError:  true,
		},
		// Explicit __typename selection.
		{
			Input:        `{ currentUser { __typename } }`,
			ExpectedRoot: `{ data: { __typename: "Query"; currentUser: (({ __typename: "User"; }) | null); }; variables: { }; }`,
			ExpectedDeclarations: GeneratedTypes{
				QueryMap: []QueryType{
					{
						Query: `{ currentUser { __typename } }`,
						Type:  `{ data: { __typename: "Query"; currentUser: (({ __typename: "User"; }) | null); }; variables: { }; }`,
					},
				},
			},
		},
		// TODO: Mutations & Subscriptions.
	}
	for _, test := range tests {
		typer := &Typer{
			Schema: schema,
		}
		filename := ""
		actualRoot, warnings, err := typer.VisitString(filename, test.Input)
		assert.Empty(t, warnings) // TODO: Test warnings.
		if test.ExpectError {
			assert.Error(t, err)
		} else if !assert.NoError(t, err, "input: %s", test.Input) {
			continue
		}
		actualDeclarations := typer.GeneratedTypes

		assert.Equal(t, test.ExpectedRoot, actualRoot)
		assert.Equal(t, test.ExpectedDeclarations, actualDeclarations)
	}
}
