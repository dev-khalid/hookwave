package main

/*

1. What would happen if reader reads from a closed channel? - Result in Zero Value of the channel's type and not block.
2. What would happen if sender sends to a closed channel? - Panic
3. How to know when a channel is closed? - By using the second value returned by the receive operation. If the channel is closed, the second value will be false.
5. Inside a goroutine sending data to a buffered channel which is already full will, and has no reader result into: Deadlock
6. From a goroutine sending data to an unbuffered channel which doesn't have any reader: Deadlock
4. Sending to buffered channel within it's size limit: Will not block the main goroutine. It will block only if the buffered channel is full and there is no reader.
7. If there's no value in a buffered channel and a reader tries to read from it, what will happen? - DeadLock
8. From within a goroutine sending data to an unbuffered channel which has a reader within that goroutine itself, what will happen? - DeadLock
9. Reader without Sender - Deadlock.

--- AI Gave me these extras ---

10. Closing an already-closed channel → panic. Closing a nil channel → panic.
11. Send/receive on a nil channel → blocks forever (not panic) — used deliberately in select to disable a case.
13. close(ch) is a broadcast — all goroutines blocked receiving on it wake up simultaneously with the zero value. This is the standard "done channel" cancellation pattern.
14. select — if multiple cases are ready, one is chosen pseudo-randomly (not first-listed). A default case makes send/receive non-blocking.
15. Directional channel types (chan<- T send-only, <-chan T receive-only) — use in function signatures to enforce who's allowed to send/close vs. only receive.
16. The deadlock detector only fires when every goroutine is blocked. If one goroutine leaks (blocked forever on a channel) but others keep running, no fatal error — it's a silent goroutine leak, not a crash. This is the actual production risk, worse than a deadlock because nothing tells you.
17. Channels have no way to check "is this closed" without receiving — no IsClosed() method; the ok idiom is the only way.
*/

/*

Intuition:
1. Channels were ment to be used for communication between goroutines.
2. Before sending to a channel, there should be a reader waiting to read from it.
*/

func main() {
	// anUnBufChannel := make(chan int)

	/*
		Use case 7: If there's no value in an unbuffered channel and a reader tries to read from it, what will happen? - DeadLock
		println("UC7: Trying to read data from an unbuffered channel which doesn't have any sender.")
		println("Result: %v", <-anUnBufChannel) // this will block the main goroutine since there is no sender
	*/

	/*
		Use case 6: From a goroutine sending data to an unbuffered channel which doesn't have any reader
		println("UC6: Trying to send data to an unbuffered channel which doesn't have any reader.")
		anUnBufChannel <- 1 // this will block the main goroutine since there is no reader
	*/

	/*
		Use Case 8: From within a goroutine sending data to an unbuffered channel which has a reader within that goroutine itself, what will happen? - DeadLock
		anUnBufChannel <- 1

		println("Result: %v", <-anUnBufChannel)
		close(anUnBufChannel) // closing the channel after sending data to it
	*/

	/*
		Use Case 1: What would happen if reader reads from a closed channel?
		go func() {
			println("Result: %v", <-anUnBufChannel)
		}()

		anUnBufChannel <- 1
		close(anUnBufChannel) // closing the channel after sending data to it

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Reader tries to read from a closed channel
			println("UC1: Trying to read data from a closed channel.")
			println("Result 2: %v", <-anUnBufChannel) // this will return the zero value of the channel's type (0 for int) and not block
		}()

		wg.Wait()
	*/

	/*
		Use Case 2: What would happen if sender sends to a closed channel?
		go func() {
			println("Result: %v", <-anUnBufChannel)
		}()

		anUnBufChannel <- 1
		close(anUnBufChannel) // closing the channel after sending data to it

		anUnBufChannel <- 2
	*/

	bch := make(chan int, 3)

	// Prepare a reciever
	go func() {
		// for v := range bch {
		// 	println("Received data from buffered channel: %v", v)
		// }

		// for {
		// 	select {
		// 	case v, ok := <-bch:
		// 		if !ok {
		// 			println("Buffered channel is closed. Exiting receiver goroutine.")
		// 			return
		// 		}
		// 		println("Received data from buffered channel: %v", v)
		// 	}
		// }

		for i := 0; i < 10; i++ {
			println("Received data from buffered channel: %v", <-bch)
		}
	}()

	// Keep sending...
	for i := 0; i < 5; i++ {
		println("Sending data to buffered channel: %v", i)
		bch <- i // this will block the main goroutine since the buffered channel is full after 3 sends
	}
	close(bch)
}
