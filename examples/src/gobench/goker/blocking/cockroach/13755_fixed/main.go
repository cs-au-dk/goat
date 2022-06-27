/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/13755
 * Buggy version: 7acb881bbb8f23e87b69fce9568d9a3316b5259c
 * fix commit-id: ef906076adc1d0e3721944829cfedfed51810088
 * Flaky: 100/100
 * Description: The buggy code does not close the db query result (rows),
 * so that one goroutine running (*Rows).awaitDone is blocked forever.
 * The blocking goroutine is waiting for cancel signal from context.
 */

package main

import (
	"context"
)

type Rows struct {
	cancel context.CancelFunc
}

func (rs *Rows) initContextClose(ctx context.Context) {
	ctx, rs.cancel = context.WithCancel(ctx)
	go rs.awaitDone(ctx) //@ go(go2)
}

func (rs *Rows) awaitDone(ctx context.Context) {
	<-ctx.Done() //@ releases(g2)
	rs.close(ctx.Err())
}

func (rs *Rows) close(err error) {
	rs.cancel()
}

/// G1 						G2
/// initContextClose()
/// 						awaitDone()
/// 						<-tx.ctx.Done()
/// return
/// ---------------G2 leak-----------------

//@ goro(main, true, _root), goro(g2, true, _root, go2)

func main() {
	rs := &Rows{}
	rs.initContextClose(context.Background())
	rs.close(nil)
}
