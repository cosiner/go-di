package di

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Logger interface {
	Begin(name string, at time.Time)
	End(name string, at time.Time, dur time.Duration)
}
type nopLogger struct{}

func (nopLogger) Begin(name string, at time.Time)                  {}
func (nopLogger) End(name string, at time.Time, dur time.Duration) {}

type Runner interface {
	run(j *Injector, p *provider, fn func() error) error
	waitDone() error
}

type syncRunner struct {
}

func SyncRunner() Runner {
	return syncRunner{}
}

func (syncRunner) run(j *Injector, p *provider, fn func() error) error {
	err := fn()
	if err != nil {
		return fmt.Errorf("%s: %s", p.name, err.Error())
	}
	return nil
}
func (syncRunner) waitDone() error {
	return nil
}

type asyncRunner struct {
	wg     sync.WaitGroup
	errors providerErrors

	mu            sync.Mutex
	providerDones map[*provider]chan struct{}
	closeCh       chan struct{}
}

func AsyncRunner() Runner {
	return &asyncRunner{
		providerDones: make(map[*provider]chan struct{}),
		closeCh:       make(chan struct{}),
	}
}

func (a *asyncRunner) getOrCreateDoneCh(p *provider) chan struct{} {
	c, has := a.providerDones[p]
	if !has {
		c = make(chan struct{})
		a.providerDones[p] = c
	}
	return c
}

func (a *asyncRunner) providerDoneCh(p *provider) chan struct{} {
	a.mu.Lock()
	c := a.getOrCreateDoneCh(p)
	a.mu.Unlock()

	return c
}

func (a *asyncRunner) doCloseCh(ch chan struct{}) {
	select {
	case <-ch:
		return
	default:
		close(ch)
	}
}
func (a *asyncRunner) finishProvider(p *provider, err error) {
	a.mu.Lock()
	c := a.getOrCreateDoneCh(p)
	if err == nil {
		close(c)
	} else {
		a.doCloseCh(a.closeCh)
		close(c)
		a.errors.Append(p.name, err)
	}
	a.mu.Unlock()
}

func (a *asyncRunner) run(j *Injector, p *provider, fn func() error) error {
	a.wg.Add(1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				e := fmt.Errorf("panic: %v, stack: %s", err, string(buf[:n]))
				a.finishProvider(p, e)
			}

			a.wg.Done()
		}()

		for _, dep := range p.deps {
			dp := j.deps.match(dep)
			if dp == nil || dp.Provider == nil {
				a.finishProvider(p, dep.notExistError(""))
				return
			}
			select {
			case <-a.providerDoneCh(dp.Provider):
			case <-a.closeCh:
				return
			}
		}

		a.finishProvider(p, fn())
	}()
	return nil
}

func (a *asyncRunner) waitDone() error {
	a.wg.Wait()
	a.mu.Lock()
	a.doCloseCh(a.closeCh)
	for _, c := range a.providerDones {
		a.doCloseCh(c)
	}
	err := a.errors.ToError()
	a.mu.Unlock()
	return err
}
