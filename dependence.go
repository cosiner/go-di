package goapp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"unicode"
)

var errorReftype = reflect.TypeOf((*error)(nil)).Elem()

type modParser interface {
	Parse(src moduleMap) (dst reflect.Value, err error)
}

type modResolver interface {
	Resolve(dst moduleMap, src reflect.Value) error
}

type modInjector interface {
	Inject(dst reflect.Value, src moduleMap) error
}

type dependence struct {
	Type reflect.Type
	Var  string
}

func (d *dependence) String() string {
	n := d.Type.String()
	if d.Var != "" {
		n += "#" + d.Var
	}
	return n
}

func (d *dependence) notExistError(provider string) error {
	n := d.String()
	if provider == "" {
		return fmt.Errorf("dependence %s not found", n)
	}
	return fmt.Errorf("dependence %s not found for provider %s", n, provider)
}

func (d *dependence) notInitializedError(provider string) error {
	n := d.String()
	if provider == "" {
		return fmt.Errorf("dependence %s not initialized", n)
	}
	return fmt.Errorf("dependence %s not initialized for provider %s", n, provider)
}

func (d *dependence) matchModuleMap(m moduleMap) *module {
	modules := m[d.Type]
	l := len(modules)
	if l == 0 {
		return nil
	}
	if l == 1 {
		return modules[0]
	}
	var def *module
	for _, mod := range modules {
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

func (d dependence) Parse(mods moduleMap) (reflect.Value, error) {
	m := d.matchModuleMap(mods)
	if m == nil {
		return reflect.Value{}, d.notExistError("")
	}
	if !m.Val.IsValid() {
		return reflect.Value{}, d.notInitializedError("")
	}
	return m.Val, nil
}

func (d dependence) Resolve(modules moduleMap, v reflect.Value) error {
	m := d.matchModuleMap(modules)
	if m == nil {
		return d.notExistError("")
	}
	m.Val = v
	return nil
}

func (d dependence) Inject(v reflect.Value, modules moduleMap) error {
	m := d.matchModuleMap(modules)
	if m == nil {
		return d.notExistError("")
	}
	v.Set(m.Val)
	return nil
}

type structFieldDep struct {
	fieldIndex int
	dependence
}

type structDependencies struct {
	reflect.Type
	fields []structFieldDep
}

func (s structDependencies) Parse(modules moduleMap) (reflect.Value, error) {
	v := reflect.New(s.Type).Elem()
	for _, d := range s.fields {
		fv, err := d.Parse(modules)
		if err != nil {
			return reflect.Value{}, err
		}
		v.Field(d.fieldIndex).Set(fv)
	}
	return v, nil
}

func (s structDependencies) Resolve(modules moduleMap, v reflect.Value) error {
	for _, d := range s.fields {
		err := d.Resolve(modules, v.Field(d.fieldIndex))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s structDependencies) Inject(v reflect.Value, modules moduleMap) error {
	for _, d := range s.fields {
		err := d.Inject(v.Field(d.fieldIndex), modules)
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

type module struct {
	dependence
	Val      reflect.Value
	Provider *provider
}

type moduleMap map[reflect.Type][]*module

type provider struct {
	name string
	fn   reflect.Value

	deps             []dependence
	depParsers       []modParser
	provides         []*module
	provideResolvers []modResolver
	errorResolver    errorResolver
}

func (p *provider) done() bool {
	return !p.fn.IsValid()
}

func (p *provider) markDone() {
	p.fn = reflect.Value{}
}

type DepDecomposable interface {
	Decompose() interface{}
}

var decomposableReftype = reflect.TypeOf((*DepDecomposable)(nil)).Elem()

type decomposableValue struct {
	Value interface{}
}

var _ DepDecomposable = decomposableValue{}

func (d decomposableValue) Decompose() interface{} {
	return d.Value
}

type namedVal struct {
	Val interface{}
	Var string
}

var namedValReftype = reflect.TypeOf((*namedVal)(nil)).Elem()

type Dependencies struct {
	running uint32

	mu        sync.RWMutex
	providers []*provider
	modules   moduleMap

	pendingMu        sync.RWMutex
	pendingProviders []interface{}
}

func NewDependencies() *Dependencies {
	return &Dependencies{
		modules: make(moduleMap),
	}
}

func (d *Dependencies) Decompose(v interface{}) interface{} {
	var dec DepDecomposable = decomposableValue{
		Value: v,
	}
	return dec
}

func (d *Dependencies) Named(name string, v interface{}) interface{} {
	return namedVal{
		Var: name,
		Val: v,
	}
}

func (d *Dependencies) parseNamed(v interface{}) namedVal {
	if nv, ok := v.(namedVal); ok {
		return nv
	}
	return namedVal{
		Val: v,
	}
}

func (d *Dependencies) analyseStruct(t reflect.Type) ([]dependence, structDependencies) {
	var s structDependencies
	s.Type = t

	l := t.NumField()
	deps := make([]dependence, 0, l)
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
		if n == "" || (unicode.IsLower(rune(n[0])) && tag == "") {
			continue
		}
		d := dependence{
			Type: ft.Type,
			Var:  ft.Name,
		}
		deps = append(deps, d)
		s.fields = append(s.fields, structFieldDep{
			fieldIndex: i,
			dependence: d,
		})
	}
	return deps, s
}

func (d *Dependencies) analyseFunc(name string, t reflect.Type, v reflect.Value) (*provider, error) {
	var p provider
	p.errorResolver.index = -1
	p.name = name
	if p.name == "" {
		p.name = functionName(v)
	}
	p.fn = v

	l := t.NumIn()
	p.deps = make([]dependence, 0, l)
	for i := 0; i < l; i++ {
		in := t.In(i)
		if in.Kind() == reflect.Struct && in.Name() == "" {
			ds, parser := d.analyseStruct(in)
			p.deps = append(p.deps, ds...)
			p.depParsers = append(p.depParsers, parser)
		} else {
			d := dependence{Type: in}
			p.deps = append(p.deps, d)
			p.depParsers = append(p.depParsers, d)
		}
	}

	l = t.NumOut()
	p.provides = make([]*module, 0, l)
	for i := 0; i < l; i++ {
		out := t.Out(i)
		if out.Kind() == reflect.Struct && out.Name() == "" {
			ds, resolver := d.analyseStruct(out)
			for _, d := range ds {
				p.provides = append(p.provides, &module{
					dependence: d,
					Provider:   &p,
				})
			}
			p.provideResolvers = append(p.provideResolvers, resolver)
		} else if out != errorReftype {
			d := dependence{Type: out}
			p.provides = append(p.provides, &module{
				dependence: d,
				Provider:   &p,
			})
			p.provideResolvers = append(p.provideResolvers, d)
		} else {
			if p.errorResolver.index >= 0 {
				return nil, fmt.Errorf("provider returned more than one error: %s", p.name)
			}
			p.errorResolver.index = i
		}
	}
	return &p, nil
}

func (d *Dependencies) analyseProvider(name string, v reflect.Value) (*provider, error) {
	t := v.Type()

	var decomposed bool
	if t.Implements(decomposableReftype) {
		decomposed = true
		pr := v.Interface().(DepDecomposable).Decompose()
		v = reflect.ValueOf(pr)
		t = v.Type()
	}

	p := &provider{
		errorResolver: errorResolver{index: -1},
	}
	switch {
	case v.Kind() == reflect.Func:
		fp, err := d.analyseFunc(name, t, v)
		if err != nil {
			return nil, err
		}
		p = fp
	case v.Kind() == reflect.Struct && (decomposed || t.Name() == ""):
		ds, resolver := d.analyseStruct(t)
		for i, d := range ds {
			p.provides = append(p.provides, &module{
				dependence: d,
				Provider:   p,
				Val:        v.Field(resolver.fields[i].fieldIndex),
			})
		}
	default:
		p.provides = append(p.provides, &module{
			dependence: dependence{Type: t, Var: name},
			Val:        v,
			Provider:   p,
		})
	}
	return p, nil
}

func (d *Dependencies) hasConflict(mods []*module, mod *module) (string, bool) {
	for _, m := range mods {
		if mod.Var == m.Var {
			return m.Provider.name, true
		}
	}
	return "", false
}

func (d *Dependencies) registerProvider(p *provider) error {
	for i := range p.provides {
		mod := p.provides[i]
		mods := d.modules[mod.Type]
		if name, conflicted := d.hasConflict(mods, mod); conflicted {
			return fmt.Errorf("provider conflicted: %s, %s, %s", name, p.name, mod.Type.String())
		}
		mods = append(mods, mod)
		if d.modules == nil {
			d.modules = make(moduleMap)
		}
		d.modules[mod.Type] = mods
	}
	d.providers = append(d.providers, p)
	return nil
}

func (d *Dependencies) provide(name string, provider reflect.Value) error {
	p, err := d.analyseProvider(name, provider)
	if err != nil {
		return err
	}
	return d.registerProvider(p)
}

func (d *Dependencies) ProvideMethods(v interface{}, pattern string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

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
			err = d.provide(functionName(m.Func), refv.Method(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Dependencies) clearPendingProviders(v []interface{}) []interface{} {
	d.pendingMu.Lock()
	v = append(v, d.pendingProviders...)
	d.pendingProviders = nil
	d.pendingMu.Unlock()
	return v
}

func (d *Dependencies) Provide(v ...interface{}) error {
	if atomic.LoadUint32(&d.running) == 0 {
		d.mu.Lock()
		defer d.mu.Unlock()

		for _, arg := range d.clearPendingProviders(v) {
			nv := d.parseNamed(arg)
			err := d.provide(nv.Var, reflect.ValueOf(nv.Val))
			if err != nil {
				return err
			}
		}
	} else {
		d.pendingMu.Lock()
		d.pendingProviders = append(d.pendingProviders, v...)
		d.pendingMu.Unlock()
	}
	return nil
}

func (d *Dependencies) runProvider(p *provider) error {
	if p.done() {
		return nil
	}
	in := make([]reflect.Value, 0, len(p.depParsers))
	for _, dp := range p.depParsers {
		v, err := dp.Parse(d.modules)
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
		err := p.provideResolvers[i].Resolve(d.modules, out[i])
		if err != nil {
			return err
		}
	}
	p.markDone()
	return nil
}

type queueNode struct {
	provider *provider
	weight   int

	parentInQueue bool
}

func (d *Dependencies) searchByProvider(queue []*queueNode, p *provider) *queueNode {
	for _, n := range queue {
		if n.provider == p {
			return n
		}
	}
	return nil
}

func (d *Dependencies) addToQueue(queue []*queueNode, p *provider, context []string) ([]*queueNode, *queueNode, error) {
	node := d.searchByProvider(queue, p)
	if p.done() {
		return queue, node, nil
	}

	if node != nil {
		if node.parentInQueue {
			return queue, node, nil
		}
		context = append(context, p.name)
		return nil, nil, fmt.Errorf("cycle dependencies: %v", context)
	}

	context = append(context, p.name)
	node = &queueNode{
		provider: p,
		weight:   1,
	}
	queue = append(queue, node)
	var (
		parent *queueNode
		err    error
	)
	for _, dep := range p.deps {
		mod := dep.matchModuleMap(d.modules)
		if mod == nil {
			return nil, nil, dep.notExistError(p.name)
		}
		queue, parent, err = d.addToQueue(queue, mod.Provider, context)
		if err != nil {
			return nil, nil, err
		}
		if parent != nil {
			node.weight += parent.weight
		}
	}
	node.parentInQueue = true
	return queue, node, nil
}

type sortbyWeight []*queueNode

func (s sortbyWeight) Len() int {
	return len(s)
}

func (s sortbyWeight) Less(i, j int) bool {
	return s[i].weight < s[j].weight
}

func (s sortbyWeight) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (d *Dependencies) buildQueue() ([]*queueNode, error) {
	var (
		// queue must be slice of node pointer for append may allocate another memory.
		queue []*queueNode
		err   error
	)
	for _, p := range d.providers {
		queue, _, err = d.addToQueue(queue, p, nil)
		if err != nil {
			return nil, err
		}
	}
	sort.Sort(sortbyWeight(queue))
	return queue, nil
}

func (d *Dependencies) checkAllDeps() error {
	var buf bytes.Buffer
	for _, p := range d.providers {
		if p.done() {
			continue
		}
		for _, dep := range p.deps {
			if dep.matchModuleMap(d.modules) == nil {
				fmt.Fprintln(&buf, dep.notExistError(p.name))
			}
		}
	}
	if buf.Len() > 0 {
		return errors.New(buf.String())
	}
	return nil
}

func (d *Dependencies) Run() error {
	if !atomic.CompareAndSwapUint32(&d.running, 0, 1) {
		return errors.New("dependencies is already running")
	}

	d.mu.Lock()
	defer func() {
		d.mu.Unlock()
		atomic.StoreUint32(&d.running, 0)
	}()

	for _, p := range d.clearPendingProviders(nil) {
		nv := d.parseNamed(p)
		err := d.provide(nv.Var, reflect.ValueOf(nv.Val))
		if err != nil {
			return err
		}
	}

	err := d.checkAllDeps()
	if err != nil {
		return err
	}

	queue, err := d.buildQueue()
	if err != nil {
		return err
	}
	for _, n := range queue {
		err = d.runProvider(n.provider)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Dependencies) inject(name string, v interface{}) error {
	refv := reflect.ValueOf(v)
	if refv.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be pointer")
	}
	refv = refv.Elem()
	dep := dependence{
		Type: refv.Type(),
		Var:  name,
	}
	mod := dep.matchModuleMap(d.modules)
	if mod != nil {
		return dep.Inject(refv, d.modules)
	}
	if refv.Kind() != reflect.Struct || dep.Type.Name() != "" {
		return dep.notExistError("")
	}
	_, r := d.analyseStruct(dep.Type)
	return r.Inject(refv, d.modules)
}

func (d *Dependencies) Inject(v ...interface{}) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, p := range v {
		nv := d.parseNamed(p)
		err := d.inject(nv.Var, nv.Val)
		if err != nil {
			return err
		}
	}
	return nil
}
