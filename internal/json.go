package internal

import "encoding/json"

func encodeString(s string) string {
	bs, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(bs)
}
