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
)

var serverInst *Server
var needQuit func() bool
var getQuit func() chan bool

func main() {
	log.Println("runtime.NumCPU():", runtime.NumCPU())
	//runtime.GOMAXPROCS(runtime.NumCPU())

	var closed int32
	quit := make(chan bool)
	needQuit = func() bool {
		select {
		case <-quit:
			return true
		default:
			return false
		}
	}
	getQuit = func() chan bool {
		return quit
	}
	setQuit := func() {
		if atom.CompareAndSwapInt32(&closed, 0, 1) {
			close(quit)
		}
	}

	var wg sync.WaitGroup

	// signal catcher
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("signal catcher goroutine quit")

		for {
			select {
			case sig := <-sigs:
				log.Println("catch signal:", sig)
				setQuit()
				return
			case <-getQuit():
				return
			}
		}
	}()

	var err error
	serverInst, err = NewServer()
	if err != nil {
		log.Printf("NewServer() returns error [%v]\n", err)
		return
	}
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
