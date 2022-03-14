const anon1 = `#graphql
  {
    hello
  }
`;

const anon2 = `#graphql
  {
    now
  }
`;

const namedFragment = `#graphql
  fragment Profile on Named {
    name
  }
`;

const badFragment = `#graphql
  { no_close_bracket_after_this
`;

// TODO: subscription
