package test

import "encoding/json"

func DeepCopy[T any](src T) T {
	var dst T
	bytes, err := json.Marshal(src)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytes, &dst)
	if err != nil {
		panic(err)
	}
	return dst
}
