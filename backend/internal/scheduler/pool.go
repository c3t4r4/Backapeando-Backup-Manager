package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerTask representa uma tarefa de backup para executar
type WorkerTask struct {
	ID        string
	ServerID  string
	BackupRun interface{} // *BackupRun (evita import circular)
	Execute   func(ctx context.Context) error
}

// WorkerPool gerencia um pool de workers para execução paralela limitada
type WorkerPool struct {
	queue       chan WorkerTask
	wg          sync.WaitGroup
	maxWorkers  int
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	closed      bool
	active      int32 // atomic: workers currently executing a task (not just queued)
	taskTimeout time.Duration
}

// NewWorkerPool cria um novo pool com limite de workers
func NewWorkerPool(maxWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		queue:      make(chan WorkerTask, maxWorkers),
		maxWorkers: maxWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetTaskTimeout bounds how long a single task's Execute may run before its
// context is canceled. Without this, a task stuck on a stalled network read
// (e.g. a hung SSH stream) occupies its worker forever — the pool's
// AvailableSlots would then never reflect that the worker is unusable,
// letting the scheduler keep enqueuing more tasks behind it (see
// docs/RegrasNegocio.md RN-BACKUP-028 and docs/Memoria.md). Zero (the
// default) disables the bound. Safe to call before or after Start.
func (p *WorkerPool) SetTaskTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskTimeout = d
}

// Start inicia o pool com numWorkers goroutines
func (p *WorkerPool) Start(numWorkers int) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	if numWorkers > p.maxWorkers {
		numWorkers = p.maxWorkers
	}

	for i := 0; i < numWorkers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker consome tarefas do canal até que a queue feche. active é
// incrementado só quando uma task está de fato em execução (não enquanto
// espera no canal) — é essa contagem, não o tamanho do canal, que
// AvailableSlots usa para refletir workers realmente ocupados.
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for task := range p.queue {
		atomic.AddInt32(&p.active, 1)
		p.runTask(task)
		atomic.AddInt32(&p.active, -1)
	}
}

// runTask executes task, bounding it with the configured task timeout (if
// any) so a hung task cannot occupy this worker forever.
func (p *WorkerPool) runTask(task WorkerTask) {
	p.mu.Lock()
	timeout := p.taskTimeout
	p.mu.Unlock()

	ctx := p.ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(p.ctx, timeout)
		defer cancel()
	}
	task.Execute(ctx)
}

// Submit enfileira uma tarefa (bloqueia se queue está cheia)
func (p *WorkerPool) Submit(task WorkerTask) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("worker pool is closed")
	}
	p.mu.Unlock()

	select {
	case p.queue <- task:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// AvailableSlots retorna quantos workers estão realmente livres (nem
// executando uma task, nem uma delas presa no canal). Antes desta correção,
// isto media apenas o espaço livre no canal (cap-len), que volta a "livre"
// assim que um worker retira a task do canal para executar — mesmo que essa
// execução dure horas. Um worker preso (rede instável, storage lento) fazia
// o scheduler continuar enfileirando novas tasks atrás dele, sem nunca serem
// processadas (ver docs/RegrasNegocio.md RN-BACKUP-028, docs/Memoria.md).
// Also subtracts tasks already sitting in the channel buffer waiting for a
// worker to pick them up (len(p.queue)) — otherwise a task submitted but not
// yet dequeued would be double-counted as "available" for a brief window,
// letting the scheduler overcommit beyond maxWorkers (Validator finding).
func (p *WorkerPool) AvailableSlots() int {
	return p.maxWorkers - int(atomic.LoadInt32(&p.active)) - len(p.queue)
}

// Stop finaliza o pool e aguarda todos os workers terminarem.
// Respeita o deadline do contexto fornecido; se o deadline expirar antes de
// todos os workers terminarem, retorna ctx.Err().
func (p *WorkerPool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	p.cancel()
	close(p.queue)

	// Create a channel to signal when wg.Wait() completes
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Wait for either wg.Wait() to complete or ctx to be canceled
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop: %w", ctx.Err())
	}
}
