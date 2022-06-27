package main

import (
	"context"
	"fmt"
)

var ErrConnClosed error

type rwmutex struct {
	w chan bool
	r chan bool
}

type Client struct {
	mu     rwmutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *Client) Close() {
	c.mu.w <- true
	defer func() {
		<-c.mu.w
	}()
	if c.cancel == nil {
		return
	}
	c.cancel()
	c.cancel = nil
	<-c.mu.w
	c.mu.w <- true // block here
}

type remoteClient struct {
	client *Client
	mu     chan bool
}

func (r *remoteClient) acquire(ctx context.Context) error {
	for {
		r.client.mu.r <- true
		closed := r.client.cancel == nil
		r.mu <- true
		<-r.mu
		if closed {
			return ErrConnClosed // Missing RUnlock before return
		}
		<-r.client.mu.r
	}
}

type kv struct {
	rc *remoteClient
}

func (kv *kv) Get(ctx context.Context) error {
	return kv.Do(ctx)
}

func (kv *kv) Do(ctx context.Context) error {
	for {
		err := kv.do(ctx)
		if err == nil {
			return nil
		}
		return err
	}
}

func (kv *kv) do(ctx context.Context) error {
	err := kv.getRemote(ctx)
	return err
}

func (kv *kv) getRemote(ctx context.Context) error {
	return kv.rc.acquire(ctx)
}

type KV interface {
	Get(ctx context.Context) error
	Do(ctx context.Context) error
}

func NewKV(c *Client) KV {
	return &kv{rc: &remoteClient{
		client: c,
		mu: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
	}}
}
func main() {
	ctx, cancel := context.WithCancel(context.TODO())
	cli := &Client{
		ctx:    ctx,
		cancel: cancel,
		mu: func() (lock rwmutex) {
			lock = rwmutex{
				w: make(chan bool),
				r: make(chan bool),
			}

			go func() {
				rCount := 0

				// As long as all locks are free, both a reader
				// and a writer may acquire the lock
				for {
					select {
					// If a writer acquires the lock, hold it until released
					case <-lock.w:
						lock.w <- false
						// If a reader acquires the lock, step into read-mode.
					case <-lock.r:
						// Increment the reader count
						rCount++
						// As long as not all readers are released, stay in read-mode.
						for rCount > 0 {
							select {
							// One reader released the lock
							case lock.r <- false:
								rCount--
								// One reader acquired the lock
							case <-lock.r:
								rCount++
							}
						}
					}
				}
			}()

			return lock
		}(),
	}
	kv := NewKV(cli)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		err := kv.Get(context.TODO())
		if err != nil && err != ErrConnClosed {
			fmt.Println("Expect ErrConnClosed")
		}
	}()

	cli.Close()

	<-donec
}
