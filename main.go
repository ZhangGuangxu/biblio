package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	atom "sync/atomic"
	"syscall"
	"time"
)

var serverInst *Server
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
	setQuit := func() {
		atom.StoreUint32(&quit, 1)
	}

	var wg sync.WaitGroup

	// signal catcher
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("signal catcher goroutine quit")

		delay := 50 * time.Millisecond
		t := time.NewTimer(delay)

		for {
			if needQuit() {
				break
			}

			t.Reset(delay)
			select {
			case sig := <-sigs:
				log.Println("catch signal:", sig)
				setQuit()
				return
			case <-t.C:
			}
		}
	}()

	serverInst = NewServer()
	wg.Add(1)
	// run the server
	go serverInst.run(&wg)

	// console command input
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ucl := strings.ToUpper(scanner.Text())
		//log.Println(ucl)
		if ucl == "QUIT" {
			setQuit()
			break
		}
		//log.Println("scanner looping ?")
	}

	log.Println("waiting for serverInst to quit ...")
	wg.Wait()
	log.Println("main goroutine quit")
}
