package main

type Gossip struct {
	mu     chan bool
	closed bool
}

func (g *Gossip) bootstrap() {
	for {
		g.mu <- true
		if g.closed {
			/// Missing g.mu.Unlock
			break
		}
		<-g.mu
		break
	}
}

func (g *Gossip) manage() {
	for {
		g.mu <- true
		if g.closed {
			/// Missing g.mu.Unlock
			break
		}
		<-g.mu
		break
	}
}
func main() {
	g := &Gossip{
		closed: true,
		mu: func() chan bool {
			ch := make(chan bool)
			go func() {
				for {
					<-ch
					ch <- false
				}
			}()
			return ch
		}(),
	}
	go func() {
		g.bootstrap()
		g.manage()
	}()
}
