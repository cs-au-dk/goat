package main

func main() {
	ch := make(chan int)

	func() {
		func() {

			func() {

				func() {

					func() {
						x := <- ch
						println(x)
					}()
				}()
			}()
		}()
	}()
}
