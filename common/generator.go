package common

type KeyedValue[T any, K any] struct {
	Key   K
	Value T
}
type YieldFn[T any, K any] func(T, K) (stopIterating bool)

type MapperFn[T any, K any] func(fn YieldFn[T, K])

type IteratorFn[T any, K any] func() (value T, key K, done bool)

type CancelFn func()

type Generator[T any, K any] struct {
	mapper  MapperFn[T, K]
	iter    IteratorFn[T, K]
	cancel  CancelFn
	started bool
}

func NewGenerator[T any, K any](mapper MapperFn[T, K]) *Generator[T, K] {
	generator := &Generator[T, K]{
		mapper: mapper,
	}
	//	generator.Start()
	return generator
}

func (p *Generator[T, K]) Start() {
	if p.started {
		return
	}
	p.started = true
	generatedValues := make(chan KeyedValue[T, K], 1)
	stopCh := make(chan interface{}, 1)
	go func() {
		p.mapper(func(obj T, key K) bool {
			select {
			case <-stopCh:
				return true
			default:
				generatedValues <- KeyedValue[T, K]{
					Key:   key,
					Value: obj,
				}
				return false
			}
		})
		close(generatedValues)
	}()
	p.iter = func() (T, K, bool) {
		value, notDone := <-generatedValues
		return value.Value, value.Key, !notDone
	}
	p.cancel = func() {
		stopCh <- nil
	}
}

func (p *Generator[T, K]) Next() (T, K, bool) {
	if !p.started {
		p.Start()
	}
	return p.iter()
}

func (p *Generator[T, K]) Cancel() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Generator[T, K]) Each(f func(value T, idx K) bool) {
	for {
		value, idx, done := p.Next()
		if done {
			break
		}
		if f(value, idx) {
			p.Cancel()
			break
		}
	}
}
