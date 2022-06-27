package main

func main() {
	ch := make(chan chan int)
	select { //@ analysis(true)
	case <-ch:
	case <-ch:
	default:
	}
	select {
	case <-ch:
	case <-ch:
	case x := <-ch:
		<-x
	default:
	}
	select {
	case <-ch:
	case <-ch:
	case <-ch:
	case <-ch:
	default:
		select {
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		}
	}
}
