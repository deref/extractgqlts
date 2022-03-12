// GENERATED FILE. DO NOT EDIT.

import { Instant } from "./scalars.ts";

export type QueryTypes = {
  "#graphql\n  {\n    hello\n  }\n": { data: { __typename: "Query"; hello: (string | null); }; variables: { }; };
  "#graphql\n  {\n    now\n  }\n": { data: { __typename: "Query"; now: Instant; }; variables: { }; };
}
