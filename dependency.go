package di

import (
	"fmt"
	"reflect"
)

type (
	dependency struct {
		Type reflect.Type
		Var  string

		Val      reflect.Value
		Provider *provider
	}

	dependencies map[reflect.Type][]*dependency

	depTool interface {
		Parse(src dependencies) (dst reflect.Value, err error)
		Resolve(dst dependencies, src reflect.Value) error
		Inject(dst reflect.Value, src dependencies) error
	}

	provider struct {
		name string
		fn   reflect.Value

		deps       []*dependency
		depParsers []depTool

		provides         []*dependency
		provideResolvers []depTool
		errorResolver    errorResolver
	}
)

func (p *provider) done() bool {
	return !p.fn.IsValid()
}

func (p *provider) markDone() {
	p.fn = reflect.Value{}
}

func (m dependencies) match(d *dependency) *dependency {
	deps := m[d.Type]
	l := len(deps)
	if l == 0 {
		return nil
	}
	if l == 1 {
		return deps[0]
	}

	var def *dependency
	for _, mod := range deps {
		if mod.Type != d.Type {
			continue
		}
		if mod.Var == d.Var {
			return mod
		}
		if mod.Var == "" {
			def = mod
		}
	}
	return def
}

func (d *dependency) String() string {
	n := d.Type.String()
	if d.Var != "" {
		n += "#" + d.Var
	}
	return n
}

func (d *dependency) notExistError(provider string) error {
	n := d.String()
	if provider == "" {
		return fmt.Errorf("dependency %s not found", n)
	}
	return fmt.Errorf("dependency %s not found for provider %s", n, provider)
}

func (d *dependency) notInitializedError(provider string) error {
	n := d.String()
	if provider == "" {
		return fmt.Errorf("dependency %s not initialized", n)
	}
	return fmt.Errorf("dependency %s not initialized for provider %s", n, provider)
}

func (d *dependency) Parse(deps dependencies) (reflect.Value, error) {
	m := deps.match(d)
	if m == nil {
		return reflect.Value{}, d.notExistError("")
	}
	if !m.Val.IsValid() {
		return reflect.Value{}, d.notInitializedError("")
	}
	return m.Val, nil
}

func (d *dependency) Resolve(deps dependencies, v reflect.Value) error {
	m := deps.match(d)
	if m == nil {
		return d.notExistError("")
	}
	m.Val = v
	return nil
}

func (d *dependency) Inject(v reflect.Value, deps dependencies) error {
	m := deps.match(d)
	if m == nil {
		return d.notExistError("")
	}
	v.Set(m.Val)
	return nil
}

type structureField struct {
	fieldIndex int
	*dependency
}

type structure struct {
	reflect.Type
	fields []structureField
}

func (s *structure) Parse(deps dependencies) (reflect.Value, error) {
	v := reflect.New(s.Type).Elem()
	for _, d := range s.fields {
		fv, err := d.Parse(deps)
		if err != nil {
			return reflect.Value{}, err
		}
		v.Field(d.fieldIndex).Set(fv)
	}
	return v, nil
}

func (s *structure) Resolve(deps dependencies, v reflect.Value) error {
	for _, d := range s.fields {
		err := d.Resolve(deps, v.Field(d.fieldIndex))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *structure) Inject(v reflect.Value, deps dependencies) error {
	for _, d := range s.fields {
		err := d.Inject(v.Field(d.fieldIndex), deps)
		if err != nil {
			return err
		}
	}
	return nil
}

type errorResolver struct {
	index int
}

func (e errorResolver) Resolve(out []reflect.Value) ([]reflect.Value, error) {
	var err error
	if e.index >= 0 {
		ef := out[e.index].Interface()
		if ef != nil {
			err = ef.(error)
		}
		if e.index != len(out)-1 {
			copy(out[e.index:], out[e.index+1:])
		}
		out = out[:len(out)-1]
	}
	return out, err
}

type optionValue struct {
	Name         string
	Decomposable bool

	Value reflect.Value
}

func parseOptionValue(v interface{}) optionValue {
	if w, ok := v.(optionValue); ok {
		return w
	}
	refv, ok := v.(reflect.Value)
	if !ok {
		refv = reflect.ValueOf(v)
	}
	return optionValue{
		Value: refv,
	}
}

func Decompose(v interface{}) interface{} {
	o := parseOptionValue(v)
	if o.Decomposable {
		return o
	}
	o.Decomposable = true
	return o
}

func Named(name string, v interface{}) interface{} {
	o := parseOptionValue(v)
	o.Name = name
	return o
}
