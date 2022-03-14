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
	// NOTE: These are essentially gold-file tests and therefore are brittle.
	tests := []struct {
		Input                string
		ExpectedRoot         string
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
					`export type Query_GetUser_Data = { __typename: "Query"; user: { __typename: "User"; bio: (string | null); name: string; }; };`,
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
					`export type Query_Fred_Data = { __typename: "Query"; named: { __typename: "Pet"; species: string; } & Fragment_Named_Data | { __typename: "Pet" | "User"; species: string; } & Fragment_Named_Data; };`,
					`export type Query_Fred_Variables = { };`,
				},
			},
		},
		// TODO: Mutations & Subscriptions.
	}
	for _, test := range tests {
		typer := &Typer{
			Schema: schema,
		}
		actualRoot, err := typer.VisitString(test.Input)
		if !assert.NoError(t, err, "input: %s", test.Input) {
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
