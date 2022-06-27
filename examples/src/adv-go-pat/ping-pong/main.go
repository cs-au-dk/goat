package main

import (
    //"fmt"
    //"time"
)

type Ball struct{ hits int }

var irrelevant chan int

func main() {
    table := make(chan *Ball)
    go player("ping", table)
    go player("pong", table)

    table <- new(Ball) // game on; toss the ball //@ analysis(true)
    //time.Sleep(1 * time.Second)
    <-irrelevant
    <-table // game over; grab the ball
}

func player(name string, table chan *Ball) {
    for {
        ball := <-table
        ball.hits++
        println(name, ball.hits)
        //time.Sleep(100 * time.Millisecond)
        <-irrelevant
        table <- ball
    }
}
