package deep

import "encoding/json"

func Copy[T any](src T) T {
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

func AlterCopy[T any](src T, alterator func(s *T)) T {
	dst := Copy(src)
	alterator(&dst)
	return dst
}

func Equal(a, b interface{}) bool {
	bytesA, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	bytesB, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}
	return string(bytesA) == string(bytesB)
}
