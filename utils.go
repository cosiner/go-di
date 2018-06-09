package di

import (
	"bytes"
	"errors"
	"fmt"
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
	name := runtime.FuncForPC(val.Pointer()).Name()
	name = trimAllPrefixByte(name, '/')
	name = strings.TrimSuffix(name, "-fm")
	return name
}

type providerErrors struct {
	buf bytes.Buffer
}

func (p *providerErrors) Append(name string, err error) {
	if p.buf.Len() > 0 {
		fmt.Fprintln(&p.buf)
	}
	fmt.Fprintf(&p.buf, "%s: %s", name, err.Error())
}
func (p *providerErrors) ToError() error {
	if p.buf.Len() > 0 {
		return errors.New(p.buf.String())
	}
	return nil
}
