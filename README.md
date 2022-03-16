# extractgqlts - Extract GraphQL TypeScript

Generates TypeScript types from GraphQL usage in string literals.

This code generation tool is intended to be extremely fast to execute.
Additionally, strong conventions make it uncomplicated to use.

## Status

**EXPERIMENTAL**

This is extremely unlike to work for you out of the box. It is being integrated
into a real product now, so that situation will slowly improve.

Feedback and/or contributions welcome.

## Install

Currently only distributed as a Go module:

```bash
go get github.com/deref/extractgqlts
```

## Usage

Write queries in your code using string literals with the following prefix:
<code>`#graphql</code>. For example:

```typescript
const { query } = './example/graphql';

const profileFragment = `#graphql
  fragment Profile on Named {
    name
  }
`

const query(`#graphql
  {
    node(id: $id) {
      id
      ...Profile
    }
  }
`, {
  include: [profileFragment],
});
```

Run the code generator, something like this:

```bash
extractgqlts \
  --schema ./src/graphql/schema.gql \
  './src/components/{tsx,svelte}' \
  > ./src/graphql/types.generated.ts
```

The generated output contains a mapped type called `QueryTypes`. This maps
query strings to `{ data, variables }` structures for use in whatever driver
functions you supply yourself. For a simple example:

```typescript
import { QueryTypes } from './types.generated.ts';

const query = <TQuery extends keyof QueryTypes>(
  query: TQuery,
  variables: QueryTypes[TQuery]['variables'],
): Promise<QueryTypes[TQuery]['data']> => {
  // ...
}
```

A more complete example can be found in [this
gist](https://gist.github.com/brandonbloom/0b2373f43d4c11f83bde3dcb61974622)
extracted from a Svelte project.

If you have custom scalars, you'll also need `./src/graphql/scalars.ts`.

## Design Constraints & Implementation Notes

### Queries Live in Component Files

It should not require many, many files to define a single UI Component. GraphQL
queries must appear in the one and only component file.

### No Manual Type Imports

Given a global schema, the query string itself should be sufficient to
determine the data and variable types. You shouldn't need to give the query an
explicit name and then laborously import a type based on that name. Until
TypeScript offers [type providers](https://github.com/microsoft/TypeScript/issues/3136),
the only way to do this without a manual import is to use a [mapped
type](https://www.typescriptlang.org/docs/handbook/2/mapped-types.html) keyed
by the query string.

### Framework Agnostic

Does not assume React, Apollo, or anything else.

This means you must provide your own entrypoint to the generated types that
indexes the `QueryTypes` map.

### Convention Over Configuration

The assumption is that you will have one module directory that will contain
three code files:

- `./types.generated.ts` - The generated output of the `extractgqlts` tool.
  Note, this name is not (yet?) enforced.
- `./scalars.ts` - Exports scalar types to be imported by `./types.generated.ts`.
- `./index.ts` - Consumes the generated types and exports exposes your
  framework-specific entrypoints.

### Global Names

Assumes there is one global namespace of query and fragment names in your
application. If you violate this, you'll get a TypeScript error regarding a
duplicate identifier.

### No TypeScript Parsing

Extracts GraphQL documents from TypeScript files by scanning for
<code>`#graphql</code>. This character sequence starts a JavaScript string
literal that is assumed to contain a GraphQL document. This pattern is also
recognized by common IDE plugins, such as [the most popular one for VS
Code](https://marketplace.visualstudio.com/items?itemName=GraphQL.vscode-graphql).

Note, we look for a string literal and not a <code>gql`</code> template literal
tag because of a [TypeScript
limitation](https://github.com/microsoft/TypeScript/issues/33304).
