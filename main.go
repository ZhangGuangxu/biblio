package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	atom "sync/atomic"
	"syscall"
	"time"
)

var needQuit func() bool

func quitChecker(quit *uint32) func() bool {
	return func() bool {
		return atom.LoadUint32(quit) == 1
	}
}

func main() {
	log.Println("runtime.NumCPU():", runtime.NumCPU())
	//runtime.GOMAXPROCS(runtime.NumCPU())

	var quit uint32
	needQuit = quitChecker(&quit)

	// signal catcher
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sigQuit := make(chan bool, 1)
	go func() {
		defer log.Println("signal catcher goroutine quit")

		for {
			if q := atom.LoadUint32(&quit); q == 1 {
				sigQuit <- true
				break
			}

			select {
			case sig := <-sigs:
				log.Println("catch signal:", sig)
				atom.StoreUint32(&quit, 1)
				sigQuit <- true
				return
			case <-time.After(50 * time.Millisecond):
				// nothing to do
			}
		}
	}()

	done := make(chan bool, 1)
	server := NewServer()
	// run the server
	go server.run(&quit, done)

	// console command input
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ucl := strings.ToUpper(scanner.Text())
		//log.Println(ucl)
		if ucl == "QUIT" {
			atom.StoreUint32(&quit, 1)
			break
		}
		//log.Println("scanner looping ?")
	}

	log.Println("waiting for sigQuit ...")
	<-sigQuit
	log.Println("waiting for listener and connections closing ...")
	<-done
	log.Println("main goroutine quit")
}
