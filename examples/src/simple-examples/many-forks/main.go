package main

func main() {
	ch := make(chan int)
	var i *int

	f := func() {
		if i == nil {
			go func() {
				ch <- 20
			}()

			i = new(int)
		} else {
			go func() {
				<-ch
			}()
			i = nil
		}
	}

	go func() {
		for {
			select {
			case ch <- 10:
				f()
			case <-ch:
			}
		}
	}()

	go func() {
		for {
			<-ch
			f()
		}
	}()
}
