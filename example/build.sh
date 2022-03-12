#!/bin/bash

go run .. \
  --schema ./schema.gql \
  './**/*.{ts}' > ./types.generated.ts
