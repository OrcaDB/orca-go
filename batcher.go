package orca

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultBatchSize is the default maximum number of predictions to
	// accumulate before sending a batch request.
	DefaultBatchSize = 10

	// DefaultBatchDelay is the default maximum time to wait for more
	// predictions before sending a batch request.
	DefaultBatchDelay = 500 * time.Millisecond
)

// BatcherOption configures a ClassificationBatcher or RegressionBatcher.
type BatcherOption func(*batcherConfig)

type batcherConfig struct {
	maxBatch int
	maxDelay time.Duration
}

// WithBatchSize sets the maximum number of predictions to accumulate before
// sending a batch request. Default is 10.
func WithBatchSize(n int) BatcherOption {
	return func(c *batcherConfig) {
		if n > 0 {
			c.maxBatch = n
		}
	}
}

// WithBatchDelay sets the maximum time to wait for more predictions before
// sending a batch request. Default is 500ms.
func WithBatchDelay(d time.Duration) BatcherOption {
	return func(c *batcherConfig) {
		if d > 0 {
			c.maxDelay = d
		}
	}
}

func newBatcherConfig(opts []BatcherOption) batcherConfig {
	cfg := batcherConfig{
		maxBatch: DefaultBatchSize,
		maxDelay: DefaultBatchDelay,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// --- Classification Batcher ---

type classifyRequest struct {
	value  string
	result chan<- classifyResult
}

type classifyResult struct {
	pred ClassificationPrediction
	err  error
}

// ClassificationBatcher accumulates individual classification predictions and
// sends them as batch requests to reduce API round trips. Predictions are
// flushed when the batch reaches the configured size or the configured delay
// elapses since the first pending item, whichever comes first.
//
// The zero value is not usable; create one with [ClassificationModel.NewBatcher].
type ClassificationBatcher struct {
	model     *ClassificationModel
	opts      []ClassifyOption
	cfg       batcherConfig
	requests  chan classifyRequest
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewBatcher creates a ClassificationBatcher that accumulates Predict calls
// and sends them as batch requests. batcherOpts configure the batch size and
// delay; classifyOpts are applied to every batch request.
//
// The caller must call [ClassificationBatcher.Close] when done to flush
// pending predictions and release resources.
func (m *ClassificationModel) NewBatcher(batcherOpts []BatcherOption, classifyOpts ...ClassifyOption) *ClassificationBatcher {
	b := &ClassificationBatcher{
		model:    m,
		opts:     classifyOpts,
		cfg:      newBatcherConfig(batcherOpts),
		requests: make(chan classifyRequest),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go b.run()
	return b
}

// Predict submits a single value for classification. It blocks until the
// batch containing this value is sent and the result is available, or until
// ctx is cancelled.
func (b *ClassificationBatcher) Predict(ctx context.Context, value string) (*ClassificationPrediction, error) {
	ch := make(chan classifyResult, 1)
	req := classifyRequest{value: value, result: ch}

	select {
	case b.requests <- req:
	case <-b.stop:
		return nil, fmt.Errorf("batcher is closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return &res.pred, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close flushes any pending predictions and stops the batcher. It blocks
// until all pending predictions have been processed. Close is safe to call
// multiple times.
func (b *ClassificationBatcher) Close() {
	b.closeOnce.Do(func() {
		close(b.stop)
	})
	<-b.done
}

func (b *ClassificationBatcher) run() {
	defer close(b.done)

	var pending []classifyRequest
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		if timer != nil {
			timer.Stop()
			timerC = nil
			timer = nil
		}
		if len(pending) == 0 {
			return
		}
		b.flushClassify(pending)
		pending = nil
	}

	for {
		select {
		case req := <-b.requests:
			pending = append(pending, req)
			if len(pending) == 1 {
				timer = time.NewTimer(b.cfg.maxDelay)
				timerC = timer.C
			}
			if len(pending) >= b.cfg.maxBatch {
				flush()
			}
		case <-timerC:
			flush()
		case <-b.stop:
			flush()
			return
		}
	}
}

func (b *ClassificationBatcher) flushClassify(items []classifyRequest) {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.value
	}

	preds, err := b.model.PredictBatch(context.Background(), values, b.opts...)
	if err != nil {
		for _, item := range items {
			item.result <- classifyResult{err: err}
		}
		return
	}

	for i, item := range items {
		if i < len(preds) {
			item.result <- classifyResult{pred: preds[i]}
		} else {
			item.result <- classifyResult{err: fmt.Errorf("prediction missing for batch index %d", i)}
		}
	}
}

// --- Regression Batcher ---

type regressRequest struct {
	value  string
	result chan<- regressResult
}

type regressResult struct {
	pred RegressionPrediction
	err  error
}

// RegressionBatcher accumulates individual regression predictions and sends
// them as batch requests to reduce API round trips. Predictions are flushed
// when the batch reaches the configured size or the configured delay elapses
// since the first pending item, whichever comes first.
//
// The zero value is not usable; create one with [RegressionModel.NewBatcher].
type RegressionBatcher struct {
	model     *RegressionModel
	opts      []RegressOption
	cfg       batcherConfig
	requests  chan regressRequest
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewBatcher creates a RegressionBatcher that accumulates Predict calls
// and sends them as batch requests. batcherOpts configure the batch size and
// delay; regressOpts are applied to every batch request.
//
// The caller must call [RegressionBatcher.Close] when done to flush
// pending predictions and release resources.
func (m *RegressionModel) NewBatcher(batcherOpts []BatcherOption, regressOpts ...RegressOption) *RegressionBatcher {
	b := &RegressionBatcher{
		model:    m,
		opts:     regressOpts,
		cfg:      newBatcherConfig(batcherOpts),
		requests: make(chan regressRequest),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go b.run()
	return b
}

// Predict submits a single value for regression. It blocks until the
// batch containing this value is sent and the result is available, or until
// ctx is cancelled.
func (b *RegressionBatcher) Predict(ctx context.Context, value string) (*RegressionPrediction, error) {
	ch := make(chan regressResult, 1)
	req := regressRequest{value: value, result: ch}

	select {
	case b.requests <- req:
	case <-b.stop:
		return nil, fmt.Errorf("batcher is closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return &res.pred, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close flushes any pending predictions and stops the batcher. It blocks
// until all pending predictions have been processed. Close is safe to call
// multiple times.
func (b *RegressionBatcher) Close() {
	b.closeOnce.Do(func() {
		close(b.stop)
	})
	<-b.done
}

func (b *RegressionBatcher) run() {
	defer close(b.done)

	var pending []regressRequest
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		if timer != nil {
			timer.Stop()
			timerC = nil
			timer = nil
		}
		if len(pending) == 0 {
			return
		}
		b.flushRegress(pending)
		pending = nil
	}

	for {
		select {
		case req := <-b.requests:
			pending = append(pending, req)
			if len(pending) == 1 {
				timer = time.NewTimer(b.cfg.maxDelay)
				timerC = timer.C
			}
			if len(pending) >= b.cfg.maxBatch {
				flush()
			}
		case <-timerC:
			flush()
		case <-b.stop:
			flush()
			return
		}
	}
}

func (b *RegressionBatcher) flushRegress(items []regressRequest) {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.value
	}

	preds, err := b.model.PredictBatch(context.Background(), values, b.opts...)
	if err != nil {
		for _, item := range items {
			item.result <- regressResult{err: err}
		}
		return
	}

	for i, item := range items {
		if i < len(preds) {
			item.result <- regressResult{pred: preds[i]}
		} else {
			item.result <- regressResult{err: fmt.Errorf("prediction missing for batch index %d", i)}
		}
	}
}
