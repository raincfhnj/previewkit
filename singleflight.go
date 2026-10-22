package previewkit

import "sync"

// group is a small in-process keyed single-flight implementation. It avoids a
// dependency on golang.org/x/sync.
type group struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

// Do runs fn once per key while concurrent callers wait for the same result.
// The third return value reports whether the result was shared.
func (g *group) Do(key string, fn func() (any, error)) (any, error, bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}

	c := &call{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	func() {
		defer c.wg.Done()
		c.val, c.err = fn()
	}()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err, false
}
