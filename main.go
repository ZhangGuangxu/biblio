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
var setQuit func()

func main() {
	log.Println("runtime.NumCPU():", runtime.NumCPU())
	//runtime.GOMAXPROCS(runtime.NumCPU())

	var quitGuard int32
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
	setQuit = func() {
		if atom.CompareAndSwapInt32(&quitGuard, 0, 1) {
			close(quit)
		}
	}

	var wg sync.WaitGroup
	startSignalHandler(&wg)

	var err error
	serverInst, err = NewServer()
	if err != nil {
		log.Printf("NewServer() returns error [%v]\n", err)
		return
	}
	serverInst.start(&wg)

	handleConsoleCommand()

	log.Println("waiting for quit...")
	wg.Wait()
	log.Println("main goroutine quit")
}

func startSignalHandler(wg *sync.WaitGroup) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Println("signal handler goroutine quit")

		for {
			select {
			case sig := <-sigs:
				log.Println("signal:", sig)
				setQuit()
				return
			case <-getQuit():
				return
			}
		}
	}()
}

func handleConsoleCommand() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ucl := strings.ToLower(scanner.Text())
		if ucl == "quit" {
			setQuit()
			break
		}
	}
}
