package main

// GoLive: replaced fmt.Println ith println
//      fmt.Scanf replaced with irrelevant channel op

import (
    //"fmt"
    "time"
)

func readword(ch chan string) {
    irrelevant := make(chan string)
    println("Type a word, then hit Enter.")
    var word string
    //fmt.Scanf("%s", &word)
    word = <-irrelevant
    ch <- word
}

func timeout(t chan bool) {
    time.Sleep(5 * time.Second)
    t <- false //@ analysis(false)
}

func main() {
    t := make(chan bool)
    go timeout(t)

    ch := make(chan string)
    go readword(ch)

    select { //@ analysis(true)
    case word := <-ch:
        println("Received", word)
    case <-t:
        println("Timeout.")
    }
}
