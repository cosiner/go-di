package goapp

import (
	"reflect"
	"runtime"
	"strings"
)

func trimAllPrefixByte(s string, b byte) string {
	for {
		index := strings.IndexByte(s, b)
		if index >= 0 {
			s = s[index+1:]
		} else {
			break
		}
	}
	return s
}

func functionName(val reflect.Value) string {
	name := trimAllPrefixByte(runtime.FuncForPC(val.Pointer()).Name(), '/')
	return name
}
