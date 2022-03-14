// GENERATED FILE. DO NOT EDIT.

import { Instant } from "./scalars";

export type Fragment_Profile_Data = { __typename: "Pet" | "User"; name: string; };
export type Fragment_Profile_Variables = { };

export type QueryTypes = {
  "#graphql\n  {\n    hello\n  }\n": { data: { __typename: "Query"; hello: string | null; }; variables: { }; };
  "#graphql\n  {\n    now\n  }\n": { data: { __typename: "Query"; now: Instant; }; variables: { }; };
  "#graphql\n  fragment Profile on Named {\n    name\n  }\n": { data: Fragment_Profile_Data; variables: Fragment_Profile_Variables; };
}
