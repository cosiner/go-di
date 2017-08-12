package di

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"
)

var errorReftype = reflect.TypeOf((*error)(nil)).Elem()

type Injector struct {
	running uint32

	mu        sync.RWMutex
	providers []*provider
	deps      dependencies

	pendingMu        sync.Mutex
	pendingProviders []interface{}
}

func New() *Injector {
	return &Injector{
		deps: make(dependencies),
	}
}

func (j *Injector) analyseStructure(t reflect.Type, provider *provider) ([]*dependency, *structure) {
	s := &structure{
		Type: t,
	}

	l := t.NumField()
	deps := make([]*dependency, 0, l)
	for i, l := 0, t.NumField(); i < l; i++ {
		ft := t.Field(i)
		tag := ft.Tag.Get("dep")
		if tag == "-" {
			continue
		}
		n := tag
		if n == "" {
			n = ft.Name
		}
		if n == "" || (!ast.IsExported(n) && tag == "") {
			continue
		}
		d := &dependency{
			Type:     ft.Type,
			Var:      ft.Name,
			Provider: provider,
		}
		deps = append(deps, d)
		s.fields = append(s.fields, structureField{
			fieldIndex: i,
			dependency: d,
		})
	}
	return deps, s
}

func (j *Injector) analyseFunc(name string, t reflect.Type, v reflect.Value) (*provider, error) {
	var p provider
	p.errorResolver.index = -1
	p.name = name
	if p.name == "" {
		p.name = functionName(v)
	}
	p.fn = v

	l := t.NumIn()
	p.deps = make([]*dependency, 0, l)
	for i := 0; i < l; i++ {
		in := t.In(i)
		if in.Kind() == reflect.Struct && in.Name() == "" {
			ds, parser := j.analyseStructure(in, nil)
			p.deps = append(p.deps, ds...)
			p.depParsers = append(p.depParsers, parser)
		} else {
			d := &dependency{Type: in}
			p.deps = append(p.deps, d)
			p.depParsers = append(p.depParsers, d)
		}
	}

	l = t.NumOut()
	p.provides = make([]*dependency, 0, l)
	for i := 0; i < l; i++ {
		out := t.Out(i)
		if out.Kind() == reflect.Struct && out.Name() == "" {
			ds, resolver := j.analyseStructure(out, &p)
			for _, d := range ds {
				p.provides = append(p.provides, d)
			}
			p.provideResolvers = append(p.provideResolvers, resolver)
		} else if out != errorReftype {
			d := dependency{Type: out, Provider: &p}
			p.provides = append(p.provides, &d)
			p.provideResolvers = append(p.provideResolvers, &d)
		} else {
			if p.errorResolver.index >= 0 {
				return nil, fmt.Errorf("provider returned more than one error: %s", p.name)
			}
			p.errorResolver.index = i
		}
	}
	return &p, nil
}

func (j *Injector) analyseProvider(val interface{}) (*provider, error) {
	o := parseOptionValue(val)
	v := o.Value
	t := v.Type()
	p := &provider{
		errorResolver: errorResolver{index: -1},
	}
	switch {
	case v.Kind() == reflect.Func:
		fp, err := j.analyseFunc(o.Name, t, v)
		if err != nil {
			return nil, err
		}
		p = fp
	case v.Kind() == reflect.Struct && (o.Decomposable || t.Name() == ""):
		ds, resolver := j.analyseStructure(t, p)
		for i, d := range ds {
			d.Val = v.Field(resolver.fields[i].fieldIndex)
		}
		p.provides = append(p.provides, ds...)
	default:
		p.provides = append(p.provides, &dependency{
			Type:     t,
			Var:      o.Name,
			Val:      v,
			Provider: p,
		})
	}
	return p, nil
}

func (j *Injector) hasConflict(mods []*dependency, mod *dependency) (string, bool) {
	for _, m := range mods {
		if mod.Var == m.Var {
			return m.Provider.name, true
		}
	}
	return "", false
}

func (j *Injector) registerProvider(p *provider) error {
	for i := range p.provides {
		mod := p.provides[i]
		mods := j.deps[mod.Type]
		if name, conflicted := j.hasConflict(mods, mod); conflicted {
			return fmt.Errorf("provider conflicted: %s, %s, %s", name, p.name, mod.Type.String())
		}
		mods = append(mods, mod)
		if j.deps == nil {
			j.deps = make(dependencies)
		}
		j.deps[mod.Type] = mods
	}
	j.providers = append(j.providers, p)
	return nil
}

func (j *Injector) provide(v interface{}) error {
	p, err := j.analyseProvider(v)
	if err != nil {
		return err
	}
	return j.registerProvider(p)
}

func (j *Injector) ProvideMethods(v interface{}, pattern string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if pattern == "" {
		pattern = ".*"
	}
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	var (
		refv = reflect.ValueOf(v)
		reft = refv.Type()
		l    = refv.Type().NumMethod()
	)
	for i := 0; i < l; i++ {
		m := reft.Method(i)
		if matcher.MatchString(m.Name) {
			err = j.provide(optionValue{
				Name:  functionName(m.Func),
				Value: refv.Method(i),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (j *Injector) clearPendingProviders(v []interface{}) []interface{} {
	j.pendingMu.Lock()
	v = append(v, j.pendingProviders...)
	j.pendingProviders = nil
	j.pendingMu.Unlock()
	return v
}

func (j *Injector) Provide(v ...interface{}) error {
	if atomic.LoadUint32(&j.running) == 0 {
		j.mu.Lock()
		defer j.mu.Unlock()

		for _, arg := range j.clearPendingProviders(v) {
			err := j.provide(arg)
			if err != nil {
				return err
			}
		}
	} else {
		j.pendingMu.Lock()
		j.pendingProviders = append(j.pendingProviders, v...)
		j.pendingMu.Unlock()
	}
	return nil
}

func (j *Injector) runProvider(p *provider) error {
	if p.done() {
		return nil
	}
	in := make([]reflect.Value, 0, len(p.depParsers))
	for _, dp := range p.depParsers {
		v, err := dp.Parse(j.deps)
		if err != nil {
			return err
		}
		in = append(in, v)
	}
	out := p.fn.Call(in)
	out, reterr := p.errorResolver.Resolve(out)
	if reterr != nil {
		return fmt.Errorf("provider %s failed with error: %s", p.name, reterr.Error())
	}
	for i := range out {
		err := p.provideResolvers[i].Resolve(j.deps, out[i])
		if err != nil {
			return err
		}
	}
	p.markDone()
	return nil
}

func (j *Injector) checkAllDeps() error {
	var buf bytes.Buffer
	for _, p := range j.providers {
		if p.done() {
			continue
		}
		for _, dep := range p.deps {
			if j.deps.match(dep) == nil {
				if buf.Len() > 0 {
					fmt.Fprintln(&buf)
				}
				fmt.Fprint(&buf, dep.notExistError(p.name))
			}
		}
	}
	if buf.Len() > 0 {
		return errors.New(buf.String())
	}
	return nil
}

func (j *Injector) Run() error {
	if !atomic.CompareAndSwapUint32(&j.running, 0, 1) {
		return errors.New("dependencies is already running")
	}

	j.mu.Lock()
	defer func() {
		j.mu.Unlock()
		atomic.StoreUint32(&j.running, 0)
	}()

	for {
		err := j.checkAllDeps()
		if err != nil {
			return err
		}

		queue, err := newQueue(j.providers, j.deps)
		if err != nil {
			return err
		}
		for _, n := range queue {
			err = j.runProvider(n.provider)
			if err != nil {
				return err
			}
		}

		providers := j.clearPendingProviders(nil)
		if len(providers) == 0 {
			break
		}
		for _, p := range providers {
			err := j.provide(p)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (j *Injector) inject(v interface{}) error {
	o := parseOptionValue(v)
	if o.Value.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be pointer")
	}
	o.Value = o.Value.Elem()
	dep := dependency{
		Type: o.Value.Type(),
		Var:  o.Name,
	}
	mod := j.deps.match(&dep)
	if mod != nil {
		return dep.Inject(o.Value, j.deps)
	}
	if o.Value.Kind() != reflect.Struct || (dep.Type.Name() != "" && !o.Decomposable) {
		return dep.notExistError("")
	}
	_, r := j.analyseStructure(dep.Type, nil)
	return r.Inject(o.Value, j.deps)
}

func (j *Injector) Inject(v ...interface{}) error {
	j.mu.RLock()
	defer j.mu.RUnlock()
	for _, p := range v {
		err := j.inject(p)
		if err != nil {
			return err
		}
	}
	return nil
}
